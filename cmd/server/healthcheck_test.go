package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestListenPort(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	if got := listenPort(cfg); got != "8080" {
		t.Fatalf("missing config: %q", got)
	}
	if err := os.WriteFile(cfg, []byte(`{"listen":"0.0.0.0:9123","scenes":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := listenPort(cfg); got != "9123" {
		t.Fatalf("from config: %q", got)
	}
	t.Setenv("SRA_LISTEN", ":8790")
	if got := listenPort(cfg); got != "8790" {
		t.Fatalf("from env: %q", got)
	}
}

func TestRunHealthcheck(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	t.Setenv("SRA_LISTEN", ":"+port)
	if code := runHealthcheck("/nonexistent"); code != 0 {
		t.Fatalf("healthy server: exit %d", code)
	}
	status = http.StatusInternalServerError
	if code := runHealthcheck("/nonexistent"); code != 1 {
		t.Fatalf("500: exit %d", code)
	}
	srv.Close()
	if code := runHealthcheck("/nonexistent"); code != 1 {
		t.Fatalf("down: exit %d", code)
	}
}
