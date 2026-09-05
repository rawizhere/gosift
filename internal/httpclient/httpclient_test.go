package httpclient

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rawizhere/gosift/internal/config"
)

func newTestClient(t *testing.T, retries int) *Client {
	t.Helper()
	cfg := &config.Config{ParseRetries: retries, ParseRetryBackoff: time.Millisecond, StoreRPS: 1000}
	c, err := New(cfg, slog.Default())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

func TestDoRetriesNotFound(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("404 page not found"))
			return
		}
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	c := newTestClient(t, 3)
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := c.Do(context.Background(), req, "komissionki")
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestDoGivesUpNotFound(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, 2)
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := c.Do(context.Background(), req, "komissionki")
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestGetBytesDoesNotRetryNotFound(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, 3)
	_, err := c.GetBytes(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("want error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
