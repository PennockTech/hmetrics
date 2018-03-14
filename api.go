// Copyright © 2018 Pennock Tech, LLC.
// All rights reserved, except as granted under license.
// Licensed per file LICENSE.txt

/*
Package hmetrics is an opinionate, simple to plumb, implementation of Heroku's
Go language metrics, which plays nicely with logs.  Heroku's implementation is
simpler if you don't want to log or have sane exponential capped backoff.
This package is simpler to use if you do.

We deliberately support neither nil logging callbacks nor callbacks being able
to cancel metrics collection via their return code.  We make a number of checks
before spawning the go-routine which does metrics posts and return those, so
the only errors afterwards will be context cancellation (your action), problems
collecting stats (should be transient) or HTTP errors posting to the endpoint,
which presumable will resolve at some point.  There's no documented guidance on
HTTP errors which indicate "service has had to move, abort and restart to
collect the new URL", so any analysis you might do in a callback is a guessing
game of little utility.

Just call Spawn() with your error-logging callback and handle the return values
from Spawn as you see fit.
*/
package hmetrics

import (
	"sync/atomic"
	"time"
)

// EnvKeyEndpoint defines the name of the environment variable defining where
// metrics should be posted to.  Its absence in environ inhibits hmetrics
// startup.
const EnvKeyEndpoint = "HEROKU_METRICS_URL"

// PackageHTTPVersion is the version string reported by default in the HTTP
// User-Agent header of our POST requests.
const PackageHTTPVersion = "0.1"

var (
	maxFailureBackoffAtomic        int64
	resetFailureBackoffAfterAtomic int64
	resetFailureBackoffToAtomic    int64
	metricsPostIntervalAtomic      int64
	httpTimeoutAtomic              int64
	httpUserAgentAtomic            atomic.Value
)

// SetMaxFailureBackoff modifies the maximum interval to which we'll back off
// between attempts to post metrics to the endpoint.
// Pass a non-zero time.Duration to modify.
// Pass 0 to make no modification.
// SetMaxFailureBackoff returns the previous value.
// SetMaxFailureBackoff is safe to call at any time from any go-routine.
func SetMaxFailureBackoff(backoff time.Duration) (previous time.Duration) {
	if backoff != 0 {
		return time.Duration(atomic.SwapInt64(&maxFailureBackoffAtomic, int64(backoff)))
	}
	return time.Duration(atomic.LoadInt64(&maxFailureBackoffAtomic))
}

// currentMaxFailureBackoff is equivalent in functionality to
// SetMaxFailureBackoff(0) but is semantically clearer to read.
func currentMaxFailureBackoff() time.Duration {
	return time.Duration(atomic.LoadInt64(&maxFailureBackoffAtomic))
}

// SetResetFailureBackoffAfter modifies the all-clear duration used to reset
// the exponential backoff in trying to start the go-routine which posts
// metrics.  If the metrics-posting Go routine lives for at least this long,
// then we consider things healthy and reset back to the value returned by
// SetResetFailureBackoffTo(0).
// Pass a non-zero time.Duration to modify.
// Pass 0 to SetResetFailureBackoffAfter to make no modification.
// SetResetFailureBackoffAfter returns the previous value.
// SetResetFailureBackoffAfter is safe to call at any time from any go-routine.
func SetResetFailureBackoffAfter(allClear time.Duration) (previous time.Duration) {
	if allClear != 0 {
		return time.Duration(atomic.SwapInt64(&resetFailureBackoffAfterAtomic, int64(allClear)))
	}
	return time.Duration(atomic.LoadInt64(&resetFailureBackoffAfterAtomic))
}

// currentResetFailureBackoffAfter is equivalent in functionality to
// SetResetFailureBackoffAfter(0) but is semantically clearer to read.
func currentResetFailureBackoffAfter() time.Duration {
	return time.Duration(atomic.LoadInt64(&resetFailureBackoffAfterAtomic))
}

// SetResetFailureBackoffTo modifies the minimum backoff period for our
// exponential backoff in trying to start the go-routine to post metrics.
// Pass a non-zero time.Duration to modify.
// Pass 0 to SetResetFailureBackoffTo to make no modification.
// SetResetFailureBackoffTo returns the previous value.
// SetResetFailureBackoffTo is safe to call at any time from any go-routine.
func SetResetFailureBackoffTo(allClear time.Duration) (previous time.Duration) {
	if allClear != 0 {
		return time.Duration(atomic.SwapInt64(&resetFailureBackoffToAtomic, int64(allClear)))
	}
	return time.Duration(atomic.LoadInt64(&resetFailureBackoffToAtomic))
}

// currentResetFailureBackoffTo is equivalent in functionality to
// SetResetFailureBackoffTo(0) but is semantically clearer to read.
func currentResetFailureBackoffTo() time.Duration {
	return time.Duration(atomic.LoadInt64(&resetFailureBackoffToAtomic))
}

// SetMetricsPostInterval modifies how often we post metrics.
// Pass a non-zero time.Duration to modify.
// Pass 0 to SetMetricsPostInterval to make no modification.
// SetMetricsPostInterval returns the previous value.
// SetMetricsPostInterval is safe to call at any time from any go-routine, but
// the value is referenced once very shortly after starting the spawned
// go-routine, so to modify this, you'll need to cancel the context of the
// metrics poster and re-Spawn.
//
// Do not use this unless you're very sure that Heroku will be happy:
// their systems will be designed around an expectation of a certain
// interval between metrics posts, and that's what we match.  You can change
// this but don't do so without explicit guidance from a Heroku employee.
func SetMetricsPostInterval(interval time.Duration) (previous time.Duration) {
	if interval != 0 {
		return time.Duration(atomic.SwapInt64(&metricsPostIntervalAtomic, int64(interval)))
	}
	return time.Duration(atomic.LoadInt64(&metricsPostIntervalAtomic))
}

// currentMetricsPostInterval is equivalent in functionality to
// SetMetricsPostInterval(0) but is semantically clearer to read.
func currentMetricsPostInterval() time.Duration {
	return time.Duration(atomic.LoadInt64(&metricsPostIntervalAtomic))
}

// SetHTTPTimeout modifies the timeout for our HTTP requests to post metrics.
// Pass a non-zero time.Duration to modify.
// Pass 0 to SetHTTPTimeout to make no modification.
// SetHTTPTimeout returns the previous value.
// SetHTTPTimeout is safe to call at any time from any go-routine.
func SetHTTPTimeout(limit time.Duration) (previous time.Duration) {
	if limit != 0 {
		return time.Duration(atomic.SwapInt64(&httpTimeoutAtomic, int64(limit)))
	}
	return time.Duration(atomic.LoadInt64(&httpTimeoutAtomic))
}

// currentHTTPTimeout is equivalent in functionality to
// SetHTTPTimeout(0) but is semantically clearer to read.
func currentHTTPTimeout() time.Duration {
	return time.Duration(atomic.LoadInt64(&httpTimeoutAtomic))
}

// SetHTTPUserAgent modifies the HTTP User-Agent header used in requests to
// post metrics to Heroku's endpoint made available to your app.
// Pass a non-empty string to set a User-Agent.  Passing an empty string will
// panic.
// SetHTTPUserAgent does not return anything.
// Use GetHTTPUserAgent to get the current value.
// SetHTTPUserAgent is safe to call at any time from any go-routine.
func SetHTTPUserAgent(ua string) {
	(&httpUserAgentAtomic).Store(ua)
}

// GetHTTPUserAgent returns the current HTTP User-Agent used in requests to
// post metrics to Heroku's endpoint made available to your app.
func GetHTTPUserAgent() string {
	return (&httpUserAgentAtomic).Load().(string)
}

/*
ErrorPoster is the function signature for the callback passed to Spawn,
and is expected to log a message based upon the error passed to it.
At its simplest:

    import "log"
	hmetrics.Spawn(func(e error) { log.Printf("hmetrics error: %s", e) })
*/
type ErrorPoster func(error)

// Spawn potentially starts the metrics-posting Go-routine.
//
// The poster parameter must not be nil, or we will error.
//
// Return values:
//
// logMessage is something worth logging as informative about what
// has happened; if the error is non-nil and you want to simplify, then ignore
// the logMessage, but it might still be helpful even with a non-nil error.
//
// cancel serves two purposes: if nil, then we did not start the go-routine, if
// non-nil then we did.  Further, if non-nil then it's a callable function
// used to cancel the context used for the go-routine posting.
//
// error is an active problem which kept us from starting.
// If we have seen an indication that logging is wanted but we do not support
// the URL specified (or could not parse it) then we will return an error.
// This should not happen in a sane environment and is probably worthy of
// a Fatal exit even if bad metrics export might normally not be, because
// your environment is messed up.
func Spawn(poster ErrorPoster) (logMessage string, cancel func(), err error) {
	return realSpawn(poster)
}

func init() {
	_ = SetMaxFailureBackoff(10 * time.Minute)
	_ = SetResetFailureBackoffAfter(5 * time.Minute)
	_ = SetResetFailureBackoffTo(time.Second)
	// Heroku use 20 seconds as the timeout for posting.  That's interesting.
	// Break compatibility.
	// Keep this strictly less than the SetMetricsPostInterval value.
	_ = SetHTTPTimeout(10 * time.Second)
	// This one is the interval Heroku used and modifying it is likely unwise
	// because it's what their systems are designed to accommodate.
	_ = SetMetricsPostInterval(20 * time.Second)
	SetHTTPUserAgent("hmetrics/" + PackageHTTPVersion + " (app using go.pennock.tech/hmetrics package)")
}
