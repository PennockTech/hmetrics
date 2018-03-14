package hmetrics

import (
	"net/url"
	"testing"
)

func TestRedacting(t *testing.T) {
	const uuid1 = "12345678-90ab-cdef-fedc-ba0987654321"
	const uuid2 = "deadbeef-abcd-9876-1234-f00fdeadbeef"
	for i, e := range []struct{ in, out string }{
		// beware with multiple query params that may be reordered
		{"http://fred:bloggs@metrics.host:1234/foo", "http://fred:redacted@metrics.host:1234/foo"},
		{"http://metrics.host:1234/foo", "http://metrics.host:1234/foo"},
		{"https://metrics.host/foo?token=wibble", "https://metrics.host/foo?token=redacted"},
		{"https://fred:bloggs@metrics.host/foo?secret=wibble", "https://fred:redacted@metrics.host/foo?secret=redacted"},
		{"https://metrics.host/" + uuid1, "https://metrics.host/redacted-uuid-form"},
		{"https://metrics.host/" + uuid1 + "/" + uuid2, "https://metrics.host/redacted-uuid-form/redacted-uuid-form"},
	} {
		dirty, err := url.Parse(e.in)
		if err != nil {
			t.Fatalf("[%d] url.Parse(%q) failed: %s", i, e.in, err)
		}
		cleaned, err := redactURL(dirty)
		if err != nil {
			t.Errorf("[%d] redactURL(%q) failed: %s", i, e.in, err)
		}
		have := cleaned.String()
		if have != e.out {
			t.Errorf("[%d] mismatch: redactURL(%q)=%q, expected %q", i, e.in, have, e.out)
		} else {
			t.Logf("[%d] good: redactURL(%q)=%q", i, e.in, have)
		}
	}
}
