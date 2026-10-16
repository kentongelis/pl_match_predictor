package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestSeasonCode(t *testing.T) {
	cases := map[int]string{
		2014: "1415",
		2009: "0910", // zero-padded
		2099: "9900", // century rollover
	}
	for start, want := range cases {
		if got := seasonCode(start); got != want {
			t.Errorf("seasonCode(%d) = %q, want %q", start, got, want)
		}
	}
}

func TestDownloadWritesAndSkipsExisting(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(r.URL.Path)) // echo the path so we can check the URL was built right
	}))
	defer srv.Close()

	fd := &FootballData{
		fetcher:     testFetcher(),
		urlTemplate: srv.URL + "/{code}/E0.csv",
		outDir:      t.TempDir(),
	}
	day := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()

	paths, err := fd.Download(ctx, []int{2014, 2015}, day)
	if err != nil {
		t.Fatalf("first Download: %v", err)
	}
	want := filepath.Join(fd.outDir, "2026-10-15", "1415.csv")
	if len(paths) != 2 || paths[0] != want {
		t.Fatalf("paths = %v, want 2 paths starting with %s", paths, want)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read %s: %v", want, err)
	}
	if string(body) != "/1415/E0.csv" {
		t.Errorf("file contents = %q, want %q", body, "/1415/E0.csv")
	}

	// Second run on the same day: nothing new, no requests.
	paths, err = fd.Download(ctx, []int{2014, 2015}, day)
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("second run wrote %v, want nothing", paths)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("server got %d requests, want 2", n)
	}
}

func TestWriteNewRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "1415.csv")
	if err := writeNew(path, []byte("original")); err != nil {
		t.Fatalf("first writeNew: %v", err)
	}
	if err := writeNew(path, []byte("replacement")); err == nil {
		t.Fatal("second writeNew: expected error, got nil")
	}

	body, _ := os.ReadFile(path)
	if string(body) != "original" {
		t.Errorf("file was overwritten: got %q", body)
	}
	// No leftover temp files.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1 (temp file left behind?)", len(entries))
	}
}
