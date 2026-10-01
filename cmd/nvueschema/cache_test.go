package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"nemith.io/nvueschema"
)

type cacheTransport func(*http.Request) (*http.Response, error)

func (f cacheTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func cacheResponse(status int, body, lastmod string) *http.Response {
	h := make(http.Header)
	if lastmod != "" {
		h.Set("Last-Modified", lastmod)
	}
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: h, Body: io.NopCloser(strings.NewReader(body))}
}
func cacheTestClient(t *testing.T, transport cacheTransport) nvueschema.VersionInfo {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	http.DefaultClient = &http.Client{Transport: transport}
	v, err := nvueschema.ParseVersion("5.16")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCacheRevalidation(t *testing.T) {
	const stamp = "Wed, 01 Jul 2026 00:00:00 GMT"
	calls := 0
	v := cacheTestClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		want := ""
		if calls == 2 || calls == 3 {
			want = stamp
		}
		if got := r.Header.Get("If-Modified-Since"); got != want {
			t.Errorf("request %d validator=%q, want %q", calls, got, want)
		}
		if r.Header.Get("User-Agent") != nvueschema.UserAgent {
			t.Error("missing User-Agent")
		}
		switch calls {
		case 1:
			return cacheResponse(200, "first", stamp), nil
		case 2:
			return cacheResponse(304, "", stamp), nil
		case 3:
			return cacheResponse(200, "updated", ""), nil
		default:
			return cacheResponse(200, "latest", stamp), nil
		}
	})
	for _, want := range []string{"first", "first", "updated", "latest"} {
		data, err := cachedFetch(v, false)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("got %q, want %q", data, want)
		}
	}
	if calls != 4 {
		t.Fatalf("made %d requests, want 4", calls)
	}
}

func TestCacheLegacyAndFailureFallback(t *testing.T) {
	calls := 0
	v := cacheTestClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return cacheResponse(200, "fresh", ""), nil
		}
		return nil, errors.New("offline")
	})
	file, stamp, err := cachePaths(v.Slug)
	if err != nil {
		t.Fatal(err)
	}
	writeCache(file, stamp, []byte("legacy"), "")
	for i := 0; i < 2; i++ {
		data, err := cachedFetch(v, false)
		if err != nil || string(data) != "fresh" {
			t.Fatalf("fetch: %q, %v", data, err)
		}
	}
	if calls != 2 {
		t.Fatalf("requests=%d, want 2", calls)
	}
}

func TestNoCacheBypassesStoredSchema(t *testing.T) {
	v := cacheTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("If-Modified-Since") != "" {
			t.Error("conditional no-cache request")
		}
		return cacheResponse(200, "fresh", "stamp"), nil
	})
	file, stamp, err := cachePaths(v.Slug)
	if err != nil {
		t.Fatal(err)
	}
	writeCache(file, stamp, []byte("stored"), "old")
	data, err := cachedFetch(v, true)
	if err != nil || string(data) != "fresh" {
		t.Fatalf("fetch: %q, %v", data, err)
	}
	cached, err := os.ReadFile(file)
	if err != nil || string(cached) != "stored" {
		t.Fatalf("cache overwritten: %q, %v", cached, err)
	}
}

func TestCacheCannotFallbackWithoutData(t *testing.T) {
	for _, status := range []int{304, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			v := cacheTestClient(t, func(r *http.Request) (*http.Response, error) { return cacheResponse(status, "", ""), nil })
			if _, err := cachedFetch(v, false); err == nil {
				t.Errorf("status %d accepted without cache", status)
			}
		})
	}
}
