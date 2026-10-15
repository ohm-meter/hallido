package radio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch_MockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/stations/search" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "fm4" {
			t.Fatalf("name=%q", r.URL.Query().Get("name"))
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "sonos-radio-agent") {
			t.Fatalf("user-agent=%q", ua)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"name":"FM4","url":"https://streams.fm4.orf.at/fm4.mp3","country":"Austria","tags":"alternative"},
			{"name":"Empty URL","url":"","country":"X"},
			{"name":"No Name","url":"https://example/stream","country":""}
		]`))
	}))
	defer srv.Close()
	c := NewClient().WithBaseURL(srv.URL).WithHTTPClient(srv.Client())
	stations, err := c.Search(context.Background(), "fm4", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 2 {
		t.Fatalf("stations=%+v", stations)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	c := NewClient()
	_, err := c.Search(context.Background(), "  ", 5)
	if err == nil {
		t.Fatal("erwarte Fehler bei leerem Query")
	}
}

func TestSearch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient().WithBaseURL(srv.URL).WithHTTPClient(srv.Client())
	_, err := c.Search(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("erwarte Fehler bei HTTP 502")
	}
}

func TestSearch_FallbackAufNaechstenMirror(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"name":"FM4","url":"https://s.example/fm4","country":"AT"}]`))
	}))
	defer good.Close()
	c := NewClient()
	c.baseURLs = []string{bad.URL, good.URL}
	st, err := c.Search(context.Background(), "fm4", 5)
	if err != nil || len(st) != 1 {
		t.Fatalf("err=%v st=%+v", err, st)
	}
}

func TestSearch_4xxKeinFallback(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	c := NewClient()
	c.baseURLs = []string{srv.URL, srv.URL}
	if _, err := c.Search(context.Background(), "x", 5); err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
