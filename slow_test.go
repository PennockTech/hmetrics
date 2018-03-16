// +build integration

package hmetrics

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicSending(t *testing.T) {
	var receivedAtomic uint64
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = ioutil.ReadAll(r.Body)
		atomic.AddUint64(&receivedAtomic, 1)
	}))

	os.Setenv(EnvKeyEndpoint, ts.URL)

	SetHTTPTimeout(100 * time.Millisecond)
	SetMetricsPostInterval(time.Second)
	SetResetFailureBackoffTo(100 * time.Millisecond)
	SetMaxFailureBackoff(2 * time.Second)
	SetResetFailureBackoffAfter(2 * time.Second)
	SetHTTPClient(ts.Client())

	Spawn(func(e error) { t.Error(e) })
	time.Sleep(3 * time.Second)
	received := atomic.LoadUint64(&receivedAtomic)
	t.Logf("server received %d requests", received)
	if received == 0 {
		t.Fail()
	}
}
