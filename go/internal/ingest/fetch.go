// Package ingest downloads raw source files into data/raw/. It never parses or modifies what it downloads;
// that's the clean stage's job.
package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/config"
)

const userAgent = "plpred/0.1"

// Fetcher does GET requests with a timeout, retries, and a rate limit. Use on Fetcher per host 
// so the rate limit applies to that host
type Fetcher struct {
	client *http.Client
	maxRetries int
	backoff time.Duration
	rateLimit time.Duration

	mu sync.Mutex // guards last
	last time.Time // when the most recent request started
}

func newFetcher(cfg config.HTTP) *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
		maxRetries: cfg.MaxRetries,
		backoff: time.Duration(cfg.BackoffIntialMS) * time.Millisecond,
		rateLimit: time.Duration(cfg.RateLimitMS) * time.Millisecond,
	}
}

// Get returns the body of the url. It retries network erros, 429s, and 5xx response up to maxRetries time,
// doubling the wait after each failure. Other statuses (like 404) fail right away, since retrying won't help
func (f *Fetcher) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		if attempt > 0 {
			// f.backoff * 2^(attempt - 1)
			delay := f.backoff << (attempt - 1) // 500ms, 1s, 2s, ...
			if err := sleep(ctx, delay); err != nil {
				return nil, err
			}
		}
		if err := f.wait(ctx); err != nil {
			return nil, err
		}

		body, retry, err := f.do(ctx, url)
		if err == nil {
			return body, nil
		}
		if !retry {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("gave up after %d attempts: %w", f.maxRetries+1, lastErr)
}

// do makes one request. retry reports whether the failure is worth retrying
func (f *Fetcher) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("build request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		// Network errors and timeouts are worth retyring, unless the caller cancelled.
		return nil, ctx.Err() == nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		retry := res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500
		return nil, true, fmt.Errorf("read body %s: %w", url, err)
	}
	return body, false, nil
}

// wait blocks until at least rateLimit has passed since the previous request
func (f *Fetcher) wait(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if d := f,rateLimit - time.Since(f.last); d > 0 {
		if err := sleep(ctx, d); err != nil {
			return err
		}
	}
	f.last = time.Now()
	return nil
}

// sleep waits for d, returning early with ctx's error if ctx is cancelled
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done();
		return ctx.Err()
	case <-t.c:
		return nil
	}
}