// Package applog is the setup and operations log (ADR 0023).
//
// Every record goes to stdout as before (journald, container log). Records at
// level Info and above are additionally kept in memory for the web app's log
// view and, if a log directory is configured (container: /data/logs),
// appended as JSON lines to a size-rotated file. Attributes whose key looks
// like a secret (token, password, authorization, ...) are replaced by
// "[redacted]" before they are stored anywhere.
package applog

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Defaults: a few hundred entries are enough to follow setup and the last
// days of operation; the files stay well below 5 MB in total.
const (
	DefaultRingSize = 500
	DefaultMaxBytes = 1 << 20 // per file
	DefaultKeep     = 3       // rotated files: hallido.log.1 … .3
	FileName        = "hallido.log"
)

// Redacted replaces the value of secret-looking attributes.
const Redacted = "[redacted]"

// Entry is one stored log record (JSON line in the file, item in the API).
type Entry struct {
	Time  time.Time         `json:"time"`
	Level string            `json:"level"`
	Msg   string            `json:"msg"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Options configures New.
type Options struct {
	Dir      string // empty = no log file (memory + stdout only)
	Debug    bool   // stdout also shows debug records
	RingSize int
	MaxBytes int64
	Keep     int
}

// Store keeps the recent entries and writes the log file.
type Store struct {
	mu       sync.Mutex
	ring     []Entry
	next     int
	full     bool
	dir      string
	file     *os.File
	size     int64
	maxBytes int64
	keep     int
}

// New creates the logger. A log directory that cannot be created or written is
// not fatal: the error is returned together with a working logger (stdout +
// memory), so the caller can log it.
func New(stdout io.Writer, opts Options) (*slog.Logger, *Store, error) {
	if opts.RingSize <= 0 {
		opts.RingSize = DefaultRingSize
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = DefaultMaxBytes
	}
	if opts.Keep <= 0 {
		opts.Keep = DefaultKeep
	}
	st := &Store{ring: make([]Entry, opts.RingSize), maxBytes: opts.MaxBytes, keep: opts.Keep}
	var fileErr error
	if opts.Dir != "" {
		fileErr = st.openDir(opts.Dir)
	}
	level := slog.LevelInfo
	if opts.Debug {
		level = slog.LevelDebug
	}
	text := slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: level, ReplaceAttr: redactAttr})
	return slog.New(&handler{next: text, store: st}), st, fileErr
}

// FileEnabled reports whether entries are written to a log file.
func (s *Store) FileEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file != nil
}

// Entries returns up to limit of the newest entries, oldest first.
func (s *Store) Entries(limit int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.next
	if s.full {
		n = len(s.ring)
	}
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]Entry, 0, limit)
	for i := n - limit; i < n; i++ {
		idx := i
		if s.full {
			idx = (s.next + i) % len(s.ring)
		}
		out = append(out, s.ring[idx])
	}
	return out
}

// Close closes the log file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

func (s *Store) openDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("log directory: %w", err)
	}
	path := filepath.Join(dir, FileName)
	s.preload(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("log file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("log file: %w", err)
	}
	s.dir, s.file, s.size = dir, f, info.Size()
	return nil
}

// preload fills the memory buffer from the existing log file, so the log view
// still shows the setup after a restart.
func (s *Store) preload(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Msg != "" {
			s.push(e)
		}
	}
}

func (s *Store) push(e Entry) {
	s.ring[s.next] = e
	s.next++
	if s.next == len(s.ring) {
		s.next = 0
		s.full = true
	}
}

func (s *Store) add(e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.push(e)
	if s.file == nil {
		return
	}
	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	line = append(line, '\n')
	if s.size+int64(len(line)) > s.maxBytes {
		s.rotate()
		if s.file == nil {
			return
		}
	}
	n, err := s.file.Write(line)
	s.size += int64(n)
	if err != nil {
		// Disk full or volume gone: keep running with stdout + memory.
		_ = s.file.Close()
		s.file = nil
	}
}

// rotate: hallido.log -> .1 -> .2 … ; the oldest file is dropped.
func (s *Store) rotate() {
	_ = s.file.Close()
	s.file = nil
	base := filepath.Join(s.dir, FileName)
	_ = os.Remove(fmt.Sprintf("%s.%d", base, s.keep))
	for i := s.keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", base, i), fmt.Sprintf("%s.%d", base, i+1))
	}
	_ = os.Rename(base, base+".1")
	f, err := os.OpenFile(base, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	s.file, s.size = f, 0
}

// handler fans a record out to stdout (next) and the store.
type handler struct {
	next   slog.Handler
	store  *Store
	attrs  []slog.Attr // from WithAttrs, keys already prefixed with groups
	prefix string      // from WithGroup ("a.b.")
}

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= slog.LevelInfo || h.next.Enabled(ctx, l)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	var err error
	if h.next.Enabled(ctx, r.Level) {
		err = h.next.Handle(ctx, r)
	}
	if r.Level < slog.LevelInfo {
		return err
	}
	e := Entry{Time: r.Time, Level: r.Level.String(), Msg: r.Message}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	attrs := make(map[string]string, len(h.attrs)+r.NumAttrs())
	for _, a := range h.attrs {
		addAttr(attrs, "", a)
	}
	r.Attrs(func(a slog.Attr) bool {
		addAttr(attrs, h.prefix, a)
		return true
	})
	if len(attrs) > 0 {
		e.Attrs = attrs
	}
	h.store.add(e)
	return err
}

func (h *handler) WithAttrs(as []slog.Attr) slog.Handler {
	c := *h
	c.next = h.next.WithAttrs(as)
	c.attrs = append([]slog.Attr{}, h.attrs...)
	for _, a := range as {
		a.Key = h.prefix + a.Key
		c.attrs = append(c.attrs, a)
	}
	return &c
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	c.next = h.next.WithGroup(name)
	c.prefix = h.prefix + name + "."
	return &c
}

// maxAttrLen caps a stored value (stream URLs, error texts).
const maxAttrLen = 500

func addAttr(dst map[string]string, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	key := prefix + a.Key
	if a.Value.Kind() == slog.KindGroup {
		for _, g := range a.Value.Group() {
			addAttr(dst, key+".", g)
		}
		return
	}
	if a.Key == "" {
		return
	}
	v := a.Value.String()
	if IsSecretKey(key) {
		v = Redacted
	}
	if len(v) > maxAttrLen {
		v = v[:maxAttrLen] + "…"
	}
	dst[key] = v
}

// IsSecretKey reports whether an attribute key names a secret.
func IsSecretKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range []string{"token", "password", "passwd", "secret", "authorization", "cookie", "api_key", "apikey"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() != slog.KindGroup && IsSecretKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	return a
}
