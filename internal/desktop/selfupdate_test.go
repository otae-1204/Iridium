package desktop

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kumoSleeping/iridium/internal/update"
)

// The app's own update: a newer release on GitHub downloaded ahead, shown as ready, put in place
// and the app started again only once asked; one on disk already only restarts.
func TestSelfUpdate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake binary is a shell script")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "iridium")
	os.WriteFile(exe, []byte("old"), 0o755)
	bin := []byte("#!/bin/sh\necho iridium v1.3.0\n")
	var tgz bytes.Buffer
	zw := gzip.NewWriter(&tgz)
	tw := tar.NewWriter(zw)
	tw.WriteHeader(&tar.Header{Name: "iridium", Mode: 0o755, Size: int64(len(bin)), Typeflag: tar.TypeReg})
	tw.Write(bin)
	tw.Close()
	zw.Close()
	archive := tgz.Bytes()
	sum := sha256.Sum256(archive)
	name := "iridium-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v1.3.0","assets":[{"name":"%s","browser_download_url":"%s/dl/%s","size":%d,"digest":"sha256:%s"}]}`,
				name, srv.URL, name, len(archive), hex.EncodeToString(sum[:]))
		case "/dl/" + name:
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	var changed, relaunched atomic.Int32
	s := newSelfUpdate(update.Options{Source: "github:o/r", GitHubAPI: srv.URL, Current: "v1.2.0", Exe: exe},
		true, func() { changed.Add(1) }, func() { relaunched.Add(1) }, t.Logf)
	s.onDisk = func() string { return "" }
	if v := s.view(); v.Phase != "" {
		t.Fatalf("before a check: %+v", v)
	}
	s.check(true)
	wait(t, func() bool { return s.view().Phase == "ready" })
	if v := s.view(); v.Version != "v1.3.0" || changed.Load() == 0 {
		t.Fatalf("ready: %+v, told %d", v, changed.Load())
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" || relaunched.Load() != 0 {
		t.Fatal("put in place, or restarted, before it was asked")
	}
	m := s.view().fields(map[string]any{})
	if m["update"] != "v1.3.0" || m["updatePct"] != nil {
		t.Fatalf("fields %v", m)
	}
	// Opened again soon: not looked for again.
	before := s.checked
	s.check(false)
	if !s.checked.Equal(before) {
		t.Error("looked again at once")
	}
	s.restart()
	wait(t, func() bool { return relaunched.Load() == 1 })
	if b, _ := os.ReadFile(exe); !bytes.Equal(b, bin) {
		t.Fatalf("not replaced: %q", b)
	}

	// Another put a newer binary on disk: shown ready, a restart only restarts.
	d := newSelfUpdate(update.Options{Source: "github:o/r", GitHubAPI: srv.URL, Current: "v1.2.0", Exe: exe},
		true, nil, func() { relaunched.Add(1) }, t.Logf)
	d.onDisk = func() string { return "v1.4.0" }
	d.check(false)
	if v := d.view(); v.Phase != "ready" || v.Version != "v1.4.0" {
		t.Fatalf("on disk: %+v", v)
	}
	d.restart()
	if relaunched.Load() != 2 {
		t.Fatal("not restarted")
	}

	// A build from source: nothing looked for.
	n := newSelfUpdate(update.Options{}, false, nil, nil, t.Logf)
	n.onDisk = func() string { return "" }
	n.check(true)
	if v := n.view(); v.Phase != "" {
		t.Fatalf("dev: %+v", v)
	}
}

func TestUpdView(t *testing.T) {
	m := updView{Phase: "downloading", Pct: 37}.fields(map[string]any{})
	if m["update"] != "" || m["updatePct"] != 37 {
		t.Fatalf("%v", m)
	}
	m = updView{Phase: "failed", Error: "boom"}.fields(map[string]any{})
	if m["update"] != "" || m["updateError"] != "boom" {
		t.Fatalf("%v", m)
	}
}

func wait(t *testing.T, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("timed out")
}
