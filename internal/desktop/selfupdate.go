package desktop

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/update"
	"github.com/kumoSleeping/iridium/internal/version"
)

// The app updates itself (docs/app.md "更新", kumo, 2026-11-26): the window looks for the latest
// release on GitHub as a server's updater does (internal/update, the same Updater), not following
// the Iridiums it is joined to. A newer one is downloaded in the background and waits; restarting
// puts it in place and starts the app again on it. This computer's Iridium, run by the runtime
// from the same binary, starts again on it too: the runtime sees its binary replaced
// (runtime.go). Release builds only.

const (
	updateEvery = 6 * time.Hour    // looked for this often
	updateAgain = 10 * time.Minute // at least this apart when the list is opened
)

// selfUpdate is the app's own update.
type selfUpdate struct {
	up       *update.Updater // nil: not a release build
	changed  func()          // what it shows changed
	relaunch func()          // the app started again on the binary on disk
	onDisk   func() string   // the version of the binary on disk ("": not known)
	logf     func(string, ...any)

	mu      sync.Mutex
	st      api.UpdateState
	disk    string // the binary on disk is this other version (put there by another): only a restart
	checked time.Time
	ready   string // the version Prepare downloaded, waiting for a restart
	last    updView
}

// updView is what is shown of it: Phase "" (nothing), "downloading" (Pct), "ready" (Version), or
// "failed" (Error).
type updView struct {
	Phase   string
	Pct     int
	Version string
	Error   string
}

// newSelfUpdate makes the app's update; o is the Updater's own (its Source the default where
// empty), changed told of what is shown changing.
func newSelfUpdate(o update.Options, release bool, changed, relaunch func(), logf func(string, ...any)) *selfUpdate {
	s := &selfUpdate{changed: changed, relaunch: relaunch, logf: logf}
	s.onDisk = s.diskVersion
	if !release {
		return s
	}
	if o.Source == "" {
		o.Source = update.DefaultSource
	}
	if o.Current == "" {
		o.Current = version.Version
	}
	o.Logf = logf
	o.Restart = func() error { s.relaunch(); return nil }
	o.Publish = func(st api.UpdateState) {
		s.mu.Lock()
		s.st = st
		s.mu.Unlock()
		s.tell()
	}
	s.up = update.New(o)
	return s
}

// run looks now, then every updateEvery. A download staged by an earlier run long ago (the app
// quit before its restart) is dropped first.
func (s *selfUpdate) run() {
	if s.up == nil {
		return
	}
	if exe, err := ownExe(); err == nil {
		update.DropStaged(exe, time.Hour)
	}
	for {
		s.check(true)
		time.Sleep(updateEvery)
	}
}

// check looks for a newer release (with !force, only if not looked for a little before): none
// while the binary on disk is already another, which a restart starts.
func (s *selfUpdate) check(force bool) {
	if v := s.onDisk(); v != "" && v != version.Version {
		s.mu.Lock()
		was := s.disk
		s.disk = v
		s.mu.Unlock()
		if was != v {
			s.tell()
		}
		return
	}
	if s.up == nil {
		return
	}
	s.mu.Lock()
	if !force && time.Since(s.checked) < updateAgain {
		s.mu.Unlock()
		return
	}
	s.checked = time.Now()
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	v, err := s.up.Prepare(ctx)
	if err != nil {
		s.logf("update: %v", err)
	}
	if err == nil {
		s.mu.Lock()
		s.ready = v
		s.mu.Unlock()
		s.tell()
	}
}

// view is what is shown now.
func (s *selfUpdate) view() updView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.viewLocked()
}

func (s *selfUpdate) viewLocked() updView {
	st := s.st
	switch {
	case s.disk != "":
		return updView{Phase: "ready", Version: s.disk}
	case st.Phase == "restarting":
		return updView{Phase: "ready", Version: st.Latest}
	case s.ready != "" && st.Phase != "downloading" && st.Phase != "failed":
		return updView{Phase: "ready", Version: s.ready}
	case st.Phase == "downloading":
		pct := 0
		if st.Size > 0 {
			pct = int(st.Got * 100 / st.Size)
		}
		return updView{Phase: "downloading", Pct: min(pct, 99), Version: st.Latest}
	case st.Phase == "failed" && st.Available:
		// A download that failed; a check that did (offline) is nothing to show.
		return updView{Phase: "failed", Version: st.Latest, Error: st.Error}
	}
	return updView{}
}

// tell says a change of what is shown (each percent of a download, not each write).
func (s *selfUpdate) tell() {
	s.mu.Lock()
	v := s.viewLocked()
	same := v == s.last
	s.last = v
	s.mu.Unlock()
	if !same && s.changed != nil {
		s.changed()
	}
}

// restart puts the update in place and starts the app again on it; one on disk already: only
// the restart. A failed one is tried again.
func (s *selfUpdate) restart() {
	s.mu.Lock()
	disk := s.disk
	s.mu.Unlock()
	if disk != "" {
		s.relaunch()
		return
	}
	if s.up == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := s.up.Apply(ctx, false); err != nil {
			s.logf("update: %v", err)
		}
	}()
}

// diskVersion asks the binary on disk its version (iridium version).
func (s *selfUpdate) diskVersion() string {
	self, err := ownExe()
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "version")
	quiet(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(string(bytes.TrimSpace(out)), "iridium "))
}

// ownExe is this program's binary, links resolved.
func ownExe() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(self); err == nil {
		self = r
	}
	return self, nil
}

// fields are what the app's pages are told of it (AppState.update, updatePct, updateError).
func (v updView) fields(m map[string]any) map[string]any {
	m["update"] = ""
	switch v.Phase {
	case "ready":
		m["update"] = version.Short(v.Version)
	case "downloading":
		m["updatePct"] = v.Pct
	case "failed":
		m["updateError"] = v.Error
	}
	return m
}
