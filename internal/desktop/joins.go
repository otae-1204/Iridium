package desktop

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/kumoSleeping/iridium/internal/join"
	"github.com/kumoSleeping/iridium/internal/remote/executor"
	"github.com/kumoSleeping/iridium/internal/remote/wire"
)

// prog is how the app says itself in its pairing and hello: the Iridium marks it as the app's.
// Update is off: the app updates as itself, not to its Iridium's release (update.go).
var prog = join.Program{Name: "iridium --join", Self: "ir-app", Usage: usage}

// Joins runs this computer's join to each Iridium that is on, each in its own goroutine, keeps
// the last thing each said for the tray, and sets on each what its owner allows, at once (Sync).
type Joins struct {
	Ask     func(context.Context, string) bool // nil: none here to ask (a grant is turned down)
	Changed func()                             // a status changed
	// AskEach asks the person here about a request from the Iridium at server, item by item (the
	// app's panel); nil: Ask.
	AskEach func(ctx context.Context, server, by string, items []executor.Item) []bool
	// Widened is told what server may do here after a request the person allowed.
	Widened func(server string, shell bool, roots, off []string)
	// Terminal: asked at the terminal it runs in (no window), as ir-computer asks, when Ask is nil.
	Terminal bool
	// Update: each join follows its Iridium's release (join.Program.Update); off where this
	// computer's own Iridium runs in the process, or the app's window updates the binary itself
	// (selfupdate.go): only executors with no screen and no server follow.
	Update bool
	// Record, when set, is kept on disk with the joins' states (iridium status).
	Record *Record
	// View: the window's. The joins run in the runtime (runtime.go), which follows the settings
	// on disk: Sync does nothing here, Status is the runtime's.
	View bool
	Logf func(string, ...any)

	mu      sync.Mutex
	running map[string]context.CancelFunc
	name    map[string]string // the computer's name each runs with: another, it starts again
	terms   map[string]Server // what each may do here now
	live    map[string]*executor.Executor
	status  map[string]string

	links map[string]Link // each join's connection, as last looked at (saveLocked)

	agentsOnce sync.Once
	agents     []wire.AgentInfo
}

// Agents are the agents installed here (looked for once).
func (j *Joins) Agents() []wire.AgentInfo {
	j.agentsOnce.Do(func() { j.agents = join.FindAgents(j.Logf) })
	return j.agents
}

// Status is what the join to address said last.
func (j *Joins) Status(address string) string {
	if j.View {
		r, ok := RuntimeRecord()
		if !ok {
			return "ir 未运行"
		}
		if st, ok := r.Joins[address]; ok {
			return st
		}
		return "连接中"
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status[address]
}

// Sync starts the joins of c's servers that are on and stops the others; on those running it
// sets their terms (command lines, folders, agents) at once, what runs there kept running.
func (j *Joins) Sync(c Config) {
	if j.View {
		return
	}
	agents := j.Agents()
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.running == nil {
		j.running, j.name, j.terms, j.live, j.status = map[string]context.CancelFunc{}, map[string]string{}, map[string]Server{}, map[string]*executor.Executor{}, map[string]string{}
	}
	want := map[string]Server{}
	for _, s := range c.Servers {
		if !s.Off && !s.Local {
			want[s.URL] = s
		}
	}
	for u, stop := range j.running {
		if _, ok := want[u]; !ok || j.name[u] != c.Name {
			stop()
			delete(j.running, u)
			delete(j.live, u)
			j.status[u] = "已断开"
		}
	}
	for u, s := range want {
		j.terms[u] = s
		if j.running[u] == nil {
			ctx, stop := context.WithCancel(context.Background())
			j.running[u], j.name[u] = stop, c.Name
			j.status[u] = "连接中"
			go j.run(ctx, s, c.Name, agents)
		} else if ex := j.live[u]; ex != nil {
			ex.Set(!s.NoShell, s.Roots, s.AgentsOff)
		}
	}
}

// Stop ends every join (the app quitting): the agents they started end with them.
func (j *Joins) Stop() {
	j.mu.Lock()
	defer j.mu.Unlock()
	for u, stop := range j.running {
		stop()
		delete(j.running, u)
	}
	if j.Record != nil {
		j.Record.remove()
	}
}

func (j *Joins) run(ctx context.Context, s Server, name string, agents []wire.AgentInfo) {
	address := s.URL
	base, err := join.BaseDir()
	if err != nil {
		j.set(address, "无法启动："+err.Error())
		return
	}
	o := join.Options{
		URL: address, Name: name, Roots: s.Roots, Shell: !s.NoShell, Base: base, Prog: j.prog(), Agents: agents, Proxy: s.Proxy,
		Out: lineWriter(func(l string) { j.set(address, l) }), Logf: j.Logf, Ask: j.askFor(address),
		Widened: func(shell bool, roots, off []string) {
			j.mu.Lock()
			t := j.terms[address]
			t.NoShell, t.Roots, t.AgentsOff = !shell, roots, off
			j.terms[address] = t
			j.mu.Unlock()
			if j.Widened != nil {
				j.Widened(address, shell, roots, off)
			}
		},
		// Each executor made (again, as it pairs again) takes the terms as they are now.
		Live: func(ex *executor.Executor) {
			j.mu.Lock()
			if ctx.Err() == nil {
				j.live[address] = ex
			}
			t := j.terms[address]
			j.mu.Unlock()
			ex.Set(!t.NoShell, t.Roots, t.AgentsOff)
		},
	}
	if j.AskEach != nil {
		o.AskEach = func(ctx context.Context, by string, items []executor.Item) []bool {
			if c, _ := loadConfig(); c.server(address) != nil && c.server(address).AlwaysAllow {
				yes := make([]bool, len(items))
				for i := range yes {
					yes[i] = true
				}
				return yes
			}
			return j.AskEach(ctx, address, by, items)
		}
	}
	if o.Name == "" {
		o.Name = join.MachineName()
	}
	if o.Ask == nil && !j.Terminal {
		// None to ask: whatever is not allowed already stays so.
		o.Ask = func(context.Context, string) bool { return false }
	}
	// A join ends on its own only for good (another runs for the same Iridium here); it is tried
	// again now and then, in case that one ends.
	for ctx.Err() == nil {
		if err := o.Run(ctx); err != nil && ctx.Err() == nil {
			j.set(address, err.Error())
		}
		select {
		case <-ctx.Done():
		case <-time.After(30 * time.Second):
		}
	}
}

func (j *Joins) prog() join.Program {
	p := prog
	p.Update = j.Update
	return p
}

func (j *Joins) set(address, line string) {
	// The terminal's wording: the app has no window to close for it.
	line, _, _ = strings.Cut(line, "：关闭此窗口即断开")
	j.mu.Lock()
	j.status[address] = line
	j.saveLocked()
	j.mu.Unlock()
	if j.Logf != nil {
		j.Logf("%s: %s", address, line)
	}
	if j.Changed != nil {
		j.Changed()
	}
}

// lineWriter calls f with each whole line written to it.
type lineWriter func(string)

func (f lineWriter) Write(p []byte) (int, error) {
	for _, l := range bytes.Split(bytes.TrimRight(p, "\n"), []byte("\n")) {
		if s := strings.TrimSpace(strings.TrimPrefix(string(l), "\a")); s != "" {
			f(s)
		}
	}
	return len(p), nil
}

// saveLocked writes the record (Record), with each join's connection as it is now.
func (j *Joins) saveLocked() {
	if j.Record == nil {
		return
	}
	if j.links == nil {
		j.links = map[string]Link{}
	}
	now := time.Now()
	for u := range j.running {
		up, rtt := false, time.Duration(0)
		if ex := j.live[u]; ex != nil {
			up, rtt = ex.Link()
		}
		l, had := j.links[u]
		if !had || l.Up != up {
			l.Since = now
		}
		l.Up, l.RTT = up, rtt.Milliseconds()
		j.links[u] = l
	}
	for u := range j.links {
		if j.running[u] == nil {
			delete(j.links, u)
		}
	}
	j.Record.Links = j.links
	j.Record.save(j.status)
}

// Save writes the record now (the runtime, every few seconds: round trips change unsaid).
func (j *Joins) Save() {
	j.mu.Lock()
	j.saveLocked()
	j.mu.Unlock()
}

// askFor is Ask for the join to address: none asked when it may have it all (AlwaysAllow).
func (j *Joins) askFor(address string) func(context.Context, string) bool {
	if j.Ask == nil {
		return nil
	}
	return func(ctx context.Context, q string) bool {
		if c, _ := loadConfig(); c.server(address) != nil && c.server(address).AlwaysAllow {
			return true
		}
		return j.Ask(ctx, q)
	}
}
