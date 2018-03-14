// Copyright © 2018 Pennock Tech, LLC.
// All rights reserved, except as granted under license.
// Licensed per file LICENSE.txt

package hmetrics

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// ErrorInvalidURL is an error type, indicating that we could not handle the
// URL which we were asked to use to post metrics.  At present, that just means
// that the scheme could not be handled but this might be extended to handle
// other scenarios we realize might cause an issue, without a semver bump.
type ErrorInvalidURL struct {
	scheme string
}

// Error is the type-satisfying method which lets an ErrorInvalidURL be an
// "error".
func (e ErrorInvalidURL) Error() string {
	return fmt.Sprintf("hmetrics: invalid URL scheme %q", e.scheme)
}

// ErrMissingPoster indicates that you've tried to not provide a callback.  We
// don't support that.  This is the one scenario for which we considered a
// library panic.  Provide a callback.  If you want to discard logable events,
// that's your decision and one which should be explicit in your code.
var ErrMissingPoster = errors.New("hmetrics: given a nil poster callback")

func realSpawn(poster ErrorPoster) (logMessage string, cancel func(), err error) {
	if poster == nil {
		return "hmetrics: not starting stats export, given no poster", nil, ErrMissingPoster
	}

	target, ok := os.LookupEnv(EnvKeyEndpoint)
	commonFailurePrefix := "hmetrics: not starting stats export, '" + EnvKeyEndpoint + "' "
	if !ok {
		return commonFailurePrefix + "not found in environ", nil, nil
	}
	if target == "" {
		return commonFailurePrefix + "is empty", nil, nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return commonFailurePrefix + "could not be parsed", nil, err
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return commonFailurePrefix + "has invalid URL scheme", nil, ErrorInvalidURL{scheme: u.Scheme}
	}

	censored, err := censorURL(u)
	if err != nil {
		return commonFailurePrefix + "is badly malformed", nil, err
	}

	// caveat: the act of censoring will re-order any query params, so the form
	// which we return for logging might not be the same as the form which we
	// use for connecting, even after accounting for the censorship.  This
	// shouldn't matter, and it would be nice to normalize the format which we
	// use to connect, but we don't dare: we need to treat the URL as being as
	// close to opaque as possible.  We're taking liberties by double-checking
	// for auth information to censor.

	ctx, cancel := context.WithCancel(context.Background())

	go retryPostLoop(ctx, u, poster)
	return fmt.Sprintf("hmetrics: started stats export to %q", censored), cancel, nil
}

func censorURL(original *url.URL) (*url.URL, error) {
	clean, err := url.Parse(original.String())
	if err != nil {
		return nil, err
	}
	if clean.User != nil {
		if _, hasPassword := clean.User.Password(); hasPassword {
			clean.User = url.UserPassword(clean.User.Username(), "censored")
		}
	}
	if clean.RawQuery == "" {
		return clean, nil
	}

	values, err := url.ParseQuery(clean.RawQuery)
	if err != nil {
		return nil, err
	}

	have := make([]string, len(values))
	i := 0
	for k := range values {
		have[i] = k
	}

	for _, k := range have {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "token") ||
			strings.Contains(lk, "password") ||
			strings.Contains(lk, "secret") ||
			strings.Contains(lk, "auth") {
			values.Set(k, "censored")
		}
	}

	clean.RawQuery = values.Encode()

	return clean, nil
}
