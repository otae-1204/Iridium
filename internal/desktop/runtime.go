package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kumoSleeping/iridium/internal/remote/executor"
)

// The runtime (docs/app.md "运行时"): ir as started (ir start), the user's service. It holds what
// the settings (the app's config, app.json) say, followed live as they change (the window's
// switches, ir server, ir join, ir leave): this computer's Iridium, a child process (iridium
// serve) while it is on, and the joins to other Iridiums. The window is only a face on it: closing
// or quitting the window leaves both running; ir stop ends them.

// RuntimeOptions are the runtime's.
type RuntimeOptions struct {
	Config string // this computer's Iridium's config file (iridium serve -config)
	Logf   func(string, ...any)
}

// Runtime runs till ctx ends.
func Runtime(ctx context.Context, o RuntimeOptions) int {
	logf := o.Logf
	self, err := os.Executable()
	if err == nil {
		self, err = filepath.EvalSymlinks(self)
	}
	if err != nil {
		logf("runtime: %v", err)
		return 1
	}
	// The joins follow their Iridiums' release only where no window of the app can be (a server
	// with no screen, an executor alone): elsewhere the app updates the binary itself from GitHub
	// (selfupdate.go, kumo, 2026-11-26), and this loop starts again on it.
	c, _ := loadConfig()
	j := &Joins{Logf: logf, Record: &Record{Kind: "runtime"}, Update: c.NoServer && !CanShow()}
	j.Ask = func(ctx context.Context, q string) bool {
		return askWindow(ctx, "", q, []executor.Item{{Kind: "text", Value: q}})[0]
	}
	j.AskEach = func(ctx context.Context, server, by string, items []executor.Item) []bool {
		return askWindow(ctx, server, by, items)
	}
	j.Widened = func(server string, shell bool, roots, off []string) {
		_, _ = updateConfig(func(c *Config) {
			if s := c.server(server); s != nil {
				s.NoShell, s.Roots, s.AgentsOff = !shell, roots, off
			}
		})
	}
	srv := &child{argv: []string{self, "serve", "-config", o.Config}, logf: logf}
	defer srv.stop()
	defer j.Stop()
	j.Record.save(nil)
	var seen time.Time
	exeTime := modTime(self)
	held := false // replaced by one macOS has not let open: said once
	for {
		if p, err := configPath(); err == nil {
			if t := modTime(p); !t.Equal(seen) || seen.IsZero() {
				seen = t
				c, err := loadConfig()
				if err != nil {
					logf("runtime: settings: %v", err)
				} else {
					srv.want(c.serverOn())
					j.Record.Server = srv.state()
					j.Sync(joinsOf(c))
				}
			}
		}
		// Replaced by an update: the new one takes over (the joins start again under it). One
		// macOS has not let open yet waits till the app is opened once: run now, it would only
		// bring up Gatekeeper's warning and end, and launchd would run it again and again.
		if t := modTime(self); !t.Equal(exeTime) && time.Since(t) > 3*time.Second {
			if unopened(self) {
				if !held {
					logf("runtime: %s replaced by one macOS has not let open yet: this one goes on till the app is opened", self)
					held = true
				}
			} else {
				logf("runtime: %s replaced: starting it again", self)
				srv.stop()
				j.Stop()
				reexec(self)
				exeTime, held = t, false
			}
		}
		j.mu.Lock()
		j.Record.Server = srv.state()
		j.mu.Unlock()
		j.Save()
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(2 * time.Second):
		}
	}
}

// joinsOf is the settings with only the Iridiums joined: never this computer's own.
func joinsOf(c Config) Config {
	out := Config{Name: c.Name}
	for _, s := range c.Servers {
		if !s.Local {
			out.Servers = append(out.Servers, s)
		}
	}
	return out
}

func modTime(p string) time.Time {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

// child is this computer's Iridium as the runtime's child: started while wanted, started again
// (after a pause) if it ends on its own, stopped when not wanted.
type child struct {
	argv []string
	logf func(string, ...any)

	mu   sync.Mutex
	on   bool
	cmd  *exec.Cmd
	done chan struct{}
	held bool // waiting for macOS to let the program open: said once (loop's own)
}

func (c *child) state() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case !c.on:
		return "已关闭"
	case c.cmd != nil:
		return "运行中"
	}
	return "启动中"
}

func (c *child) want(on bool) {
	c.mu.Lock()
	was := c.on
	c.on = on
	c.mu.Unlock()
	switch {
	case on && !was:
		go c.loop()
	case !on && was:
		c.kill()
	}
}

func (c *child) stop() {
	c.mu.Lock()
	c.on = false
	c.mu.Unlock()
	c.kill()
}

func (c *child) kill() {
	c.mu.Lock()
	cmd, done := c.cmd, c.done
	c.mu.Unlock()
	if cmd == nil {
		return
	}
	terminate(cmd)
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func (c *child) loop() {
	for {
		c.mu.Lock()
		if !c.on || c.cmd != nil {
			c.mu.Unlock()
			return
		}
		if unopened(c.argv[0]) {
			// Replaced by one macOS has not let open: started, it would only bring up the warning.
			c.mu.Unlock()
			if !c.held {
				c.logf("runtime: server: %s not let open by macOS yet: waiting till the app is opened", c.argv[0])
				c.held = true
			}
			time.Sleep(5 * time.Second)
			continue
		}
		c.held = false
		cmd := exec.Command(c.argv[0], c.argv[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		quiet(cmd)
		if err := cmd.Start(); err != nil {
			c.mu.Unlock()
			c.logf("runtime: server: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		done := make(chan struct{})
		c.cmd, c.done = cmd, done
		c.mu.Unlock()
		err := cmd.Wait()
		close(done)
		c.mu.Lock()
		c.cmd = nil
		on := c.on
		c.mu.Unlock()
		if !on {
			return
		}
		c.logf("runtime: server ended (%v): starting it again", err)
		time.Sleep(2 * time.Second)
	}
}
