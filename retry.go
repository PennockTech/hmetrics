// Copyright © 2018 Pennock Tech, LLC.
// All rights reserved, except as granted under license.
// Licensed per file LICENSE.txt

package hmetrics

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/pkg/errors"
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
		err = errors.WithMessage(err, fmt.Sprintf("hmetrics postLoop lasted %.2fms", float64(duration)/float64(time.Microsecond)))

		if duration >= currentResetFailureBackoffAfter() {
			backoff = currentResetFailureBackoffTo()
		}
		max := currentMaxFailureBackoff()
		if backoff > max {
			backoff = max
		}

		if isDeadContext(ctx) {
			poster(errors.WithMessage(err, "hmetrics retryPostLoop exiting too, context cancelled"))
			return
		}

		poster(errors.WithMessage(err, fmt.Sprintf("sleeping %.2fs", float64(backoff)/float64(time.Second))))

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			poster(errors.Wrap(ctx.Err(), "hmetrics: context cancelled while in delay backoff, exiting"))
			// nb: Leaks the channel, unless raced and already exited.
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}

	}
}
