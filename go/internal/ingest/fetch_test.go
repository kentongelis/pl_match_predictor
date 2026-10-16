package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/kentongelis/match_predictor/go/internal/config"
)

// testFetcher uses tiny delays so retry tests run fast.
func testFetcher() *Fetcher {
	return newFetcher(config.HTTP{
		TimeoutSeconds:   5,
		MaxRetries:       3,
		BackoffInitialMS: 1,
		RateLimitMS:      0,
	})
}

func TestGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent = %q, want %q", got, userAgent)
		}
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	body, err := testFetcher().Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", body, "hello")
	}
}

func TestGetRetriesServerErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail twice with 503, then succeed.
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	body, err := testFetcher().Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("server got %d requests, want 3", n)
	}
}

func TestGetNoRetryOn404(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := testFetcher().Get(context.Background(), srv.URL); err == nil {
		t.Fatal("Get: expected error for 404, got nil")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("server got %d requests, want 1 (404 must not be retried)", n)
	}
}
