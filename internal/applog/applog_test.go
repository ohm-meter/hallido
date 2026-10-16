package applog

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStore_MemoryOnly_RedactsAndOrders(t *testing.T) {
	var out bytes.Buffer
	log, st, err := New(&out, Options{RingSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	log.Debug("hidden")
	log.Info("first", "api_token", "s3cret", "speaker", "Kitchen")
	log.With("component", "setup").WithGroup("req").Warn("second", "Authorization", "Bearer x", "ip", "10.0.0.2")
	log.Error("third", "err", errors.New("boom"))
	log.Info("fourth")

	if st.FileEnabled() {
		t.Fatal("no directory -> no file")
	}
	got := st.Entries(0)
	if len(got) != 3 || got[0].Msg != "second" || got[2].Msg != "fourth" {
		t.Fatalf("ring order wrong: %+v", got)
	}
	if got[0].Attrs["component"] != "setup" || got[0].Attrs["req.ip"] != "10.0.0.2" {
		t.Fatalf("attrs/groups wrong: %+v", got[0].Attrs)
	}
	if got[0].Attrs["req.Authorization"] != Redacted {
		t.Fatalf("authorization not redacted: %+v", got[0].Attrs)
	}
	if got[0].Level != "WARN" || got[1].Attrs["err"] != "boom" {
		t.Fatalf("level/err wrong: %+v", got)
	}
	if last := st.Entries(1); len(last) != 1 || last[0].Msg != "fourth" {
		t.Fatalf("limit wrong: %+v", last)
	}
	if strings.Contains(out.String(), "s3cret") || strings.Contains(out.String(), "Bearer x") {
		t.Fatalf("secret on stdout: %s", out.String())
	}
	if strings.Contains(out.String(), "hidden") {
		t.Fatal("debug shown without Debug option")
	}
}

func TestStore_FileRotationAndPreload(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	log, st, err := New(&bytes.Buffer{}, Options{Dir: dir, MaxBytes: 300, Keep: 2})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		log.Info("entry", "n", i, "token", "never-on-disk")
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{FileName, FileName + ".1", FileName + ".2"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		if info.Size() > 300 {
			t.Fatalf("%s larger than limit: %d", name, info.Size())
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode %v, want 0600", name, info.Mode().Perm())
		}
		raw, _ := os.ReadFile(filepath.Join(dir, name))
		if strings.Contains(string(raw), "never-on-disk") {
			t.Fatalf("secret written to %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, FileName+".3")); err == nil {
		t.Fatal("more rotated files than Keep")
	}

	// Restart: the memory buffer is filled from the current file.
	_, st2, err := New(&bytes.Buffer{}, Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	got := st2.Entries(0)
	if len(got) == 0 || got[len(got)-1].Attrs["n"] != "19" {
		t.Fatalf("preload wrong: %+v", got)
	}
}

func TestNew_UnwritableDirIsNotFatal(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	log, st, err := New(&bytes.Buffer{}, Options{Dir: filepath.Join(blocker, "logs")})
	if err == nil {
		t.Fatal("expected an error for an unusable directory")
	}
	log.Info("still works")
	if st.FileEnabled() || len(st.Entries(0)) != 1 {
		t.Fatal("logger must fall back to memory")
	}
}

func TestIsSecretKey(t *testing.T) {
	for _, k := range []string{"api_token", "Token", "password", "Authorization", "client_secret"} {
		if !IsSecretKey(k) {
			t.Errorf("%q should be secret", k)
		}
	}
	for _, k := range []string{"speaker", "scene", "took", "ip", "msg"} {
		if IsSecretKey(k) {
			t.Errorf("%q should not be secret", k)
		}
	}
}
