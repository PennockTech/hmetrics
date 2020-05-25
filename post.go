// Copyright © 2018,2020 Pennock Tech, LLC.
// All rights reserved, except as granted under license.
// Licensed per file LICENSE.txt
//
// Portions "Copyright (c) 2018 Salesforce" and under BSD 3-clause license --
// specifically, the exact naming of the struct items posted, although comments
// in the code indicate that the logic was cribbed from
// https://github.com/codahale/metrics/blob/master/runtime/memstats.go which is
// under a MIT license and "Copyright (c) 2014 Coda Hale" so I don't know who
// actually has ownership of the code which is copied for compatibility with
// service expectations.

package hmetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

func postLoop(ctx context.Context, metricsURL *url.URL, poster ErrorPoster) error {
	// we tick once every 20 seconds, so Heroku should get exactly 3 posts
	// per minute, except that their logic allows 20 seconds for HTTP
	// timeout, so they can then catch up with the next ticker immediately
	// or discover that one times out too ... so there's no way to be sure
	// the metrics are evenly spaced.
	//
	// Also, if we fail to collect metrics, then we will skip that post.
	ourTickerDuration := currentMetricsPostInterval()
	maxSanePostDuration := ourTickerDuration - time.Second
	intervalTicker := time.NewTicker(ourTickerDuration)
	// unlike a Timer, a Ticker has no need to drain it?
	defer intervalTicker.Stop()

	var buf bytes.Buffer
	var pauseTotalNS uint64
	var numGC uint32
	var err error

	httpClient := GetHTTPClient()
	httpClient.Timeout = currentHTTPTimeout()

	if httpClient.Timeout > maxSanePostDuration {
		httpClient.Timeout = maxSanePostDuration
		_ = SetHTTPTimeout(maxSanePostDuration)
	}

	for {
		select {
		case <-intervalTicker.C:
		case <-ctx.Done():
			return ctx.Err()
		}

		cht := currentHTTPTimeout()
		if cht > maxSanePostDuration {
			_ = SetHTTPTimeout(maxSanePostDuration)
			cht = maxSanePostDuration
		}
		if cht != httpClient.Timeout {
			httpClient.Timeout = cht
		}

		buf.Reset()
		pauseTotalNS, numGC, err = gatherMetrics(&buf, pauseTotalNS, numGC)
		if err != nil {
			poster(err)
			continue
		}

		// I wonder what a random _short_ sleep (under 2ms) would do here, to
		// help avoid lock-step sync?  We'd have _collected_ the metrics at a
		// perfectly regular interval and I don't think Heroku's metrics are at
		// fine enough resolution for it to matter.
		// For now, match Heroku, no sleep.
		if err = submitMetrics(ctx, httpClient, &buf, metricsURL); err != nil {
			poster(err)
		}
	}
}

// This is currently copied unmodified from Heroku's code so is under their
// (Salesforce's) copyright, as noted at the top of this file, unless (as noted
// there) it's under Coda Hale's copyright.
//
// We pretty much have to copy/paste, because this is the interface schema for
// talking to their service and the code is the _only_ public documentation (at
// time of writing) of what needs to be posted, so this has to match precisely.
//
// Salesforce-Copyright: {{{
func gatherMetrics(w io.Writer, prevPauseTotalNS uint64, prevNumGC uint32) (uint64, uint32, error) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// cribbed from https://github.com/codahale/metrics/blob/master/runtime/memstats.go
	result := struct {
		Counters map[string]float64 `json:"counters"`
		Gauges   map[string]float64 `json:"gauges"`
	}{
		Counters: map[string]float64{
			"go.gc.collections": float64(stats.NumGC - prevNumGC),
			"go.gc.pause.ns":    float64(stats.PauseTotalNs - prevPauseTotalNS),
		},
		Gauges: map[string]float64{
			"go.memory.heap.bytes":   float64(stats.Alloc),
			"go.memory.stack.bytes":  float64(stats.StackInuse),
			"go.memory.heap.objects": float64(stats.Mallocs - stats.Frees), // Number of "live" objects.
			"go.gc.goal":             float64(stats.NextGC),                // Goal heap size for next GC.
			"go.routines":            float64(runtime.NumGoroutine()),      // Current number of goroutines.
		},
	}

	return stats.PauseTotalNs, stats.NumGC, json.NewEncoder(w).Encode(result)
}

// Salesforce-Copyright: }}}

// This was also copy/paste but this is also so formulaic that it's what anyone
// would have written anyway.  The only point to decide is what value to use
// for the Content-Type header.  Plus how to construct the error, which we did
// actually change.  And we adjusted the req context pairing, to make this
// closer to my style (associated the ctx ASAP to match conceptually those
// functions which take a ctx when generating).  And added a User-Agent.
func submitMetrics(ctx context.Context, client *http.Client, r io.Reader, metricsURL *url.URL) error {
	req, err := http.NewRequest("POST", metricsURL.String(), r)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", GetHTTPUserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		safe, err := redactURL(metricsURL)
		if err != nil {
			safe = metricsURL
		}
		return HTTPFailureError{
			ExpectedResponseCode: http.StatusOK,
			ActualResponseCode:   resp.StatusCode,
			URL:                  safe.String(),
		}
	}

	return nil
}
