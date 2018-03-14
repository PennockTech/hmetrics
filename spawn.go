// Copyright © 2018 Pennock Tech, LLC.
// All rights reserved, except as granted under license.
// Licensed per file LICENSE.txt

package hmetrics

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// InvalidURLError is an error type, indicating that we could not handle the
// URL which we were asked to use to post metrics.  At present, that just means
// that the scheme could not be handled but this might be extended to handle
// other scenarios we realize might cause an issue, without a semver bump.
type InvalidURLError struct {
	scheme string
}

// Error is the type-satisfying method which lets an InvalidURLError be an
// "error".
func (e InvalidURLError) Error() string {
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
		return commonFailurePrefix + "has invalid URL scheme", nil, InvalidURLError{scheme: u.Scheme}
	}

	redacted, err := redactURL(u)
	if err != nil {
		return commonFailurePrefix + "is badly malformed", nil, err
	}

	// caveat: the act of redacting will re-order any query params, so the form
	// which we return for logging might not be the same as the form which we
	// use for connecting, even after accounting for the redaction.  This
	// shouldn't matter, and it would be nice to normalize the format which we
	// use to connect, but we don't dare: we need to treat the URL as being as
	// close to opaque as possible.  We're taking liberties by double-checking
	// for auth information to redact.

	ctx, cancel := context.WithCancel(context.Background())

	go retryPostLoop(ctx, u, poster)
	return fmt.Sprintf("hmetrics: started stats export to %q", redacted), cancel, nil
}

var uuidRegexp *regexp.Regexp

func init() {
	// string layout form per RFC4122
	//
	// This will over-match, because `\b` matches between hex and '-', but we
	// want to use ReplaceAllString sanely and RE2 doesn't support negative
	// look ahead assertions or resetting the match point, so if we match on a
	// `/` at the end, that will keep the `/` from being considered as the
	// start of the next sequence, and "uuid/uuid" will only detect the first.
	uuidRegexp = regexp.MustCompile(`(^|/)([0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12})\b`)
}

func redactURL(original *url.URL) (*url.URL, error) {
	clean, err := url.Parse(original.String())
	if err != nil {
		return nil, err
	}
	if clean.User != nil {
		if _, hasPassword := clean.User.Password(); hasPassword {
			clean.User = url.UserPassword(clean.User.Username(), "redacted")
		}
	}
	if clean.Path != "/" && clean.Path != "" && uuidRegexp.MatchString(clean.Path) {
		clean.Path = uuidRegexp.ReplaceAllString(clean.Path, "${1}redacted-uuid-form")
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
			values.Set(k, "redacted")
		}
	}

	clean.RawQuery = values.Encode()

	return clean, nil
}
