// Copyright © 2018,2020 Pennock Tech, LLC.
// All rights reserved, except as granted under license.
// Licensed per file LICENSE.txt

package hmetrics

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"
)

// raiseBackoff is the basic exponential backoff for the retry loop, but with
// jitter thrown in because jitter helps avoid lock-step synchronization and
// failures therefrom.
func raiseBackoff(b time.Duration) time.Duration {
	b *= 2
	b += time.Duration(rand.Int63n(500)) * time.Millisecond
	return b
}

func isDeadContext(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// retryPostLoop should be the top function in a new go-routine
func retryPostLoop(ctx context.Context, u *url.URL, poster ErrorPoster) {
	for backoff := currentResetFailureBackoffTo(); ; backoff = raiseBackoff(backoff) {
		var err error
		if isDeadContext(ctx) {
			poster(ctx.Err())
			return
		}

		startLatest := time.Now()
		err = postLoop(ctx, u, poster)
		duration := time.Since(startLatest)

		if err == nil {
			// the only error which _can_ be returned, at time of writing, is one indicating context cancellation.
			err = errors.New("exited strangely")
		}
		err = fmt.Errorf("hmetrics postLoop lasted %.2fms: %w", float64(duration)/float64(time.Microsecond), err)

		if duration >= currentResetFailureBackoffAfter() {
			backoff = currentResetFailureBackoffTo()
		}
		max := currentMaxFailureBackoff()
		if backoff > max {
			backoff = max
		}

		if isDeadContext(ctx) {
			poster(fmt.Errorf("hmetrics retryPostLoop exiting too, context cancelled: %w", err))
			return
		}

		poster(fmt.Errorf("sleeping %.2fs: %w", float64(backoff)/float64(time.Second), err))

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			// When I used pkg/errors, I used errors.Wrap here, so a failure would include a stack trace.
			// We've lost that with a return to stdlib errors (now that %w is supported).
			// If Go's standard error handled expands to support that style of stack-trace-included error,
			// switch to it.
			poster(fmt.Errorf("hmetrics: context cancelled while in delay backoff, exiting: %w", ctx.Err()))
			// nb: Leaks the channel, unless raced and already exited.
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}

	}
}
