package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kumoSleeping/iridium/internal/agent"
	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/ctx/assemble"
	"github.com/kumoSleeping/iridium/internal/transcript"
)

// What the carried states hold beyond the session's own fields (plan "实时数据（携带）"): the
// newest blocks while they are small, the context's size, and the quota windows per agent and
// account, kept with their age when no process of theirs runs.

const (
	fileContext = "context.json" // the last get_context_usage answer
	fileQuota   = "quota.json"   // in the state directory: the last windows per agent and account
	// TailBytes: the most of the transcript a tail is read from; TailJSON: the most a tail's
	// blocks may weigh. Past either the page fetches.
	TailBytes = 256 << 10
	TailJSON  = 16 << 10
	// tailSpan: a tail reaches back at least this far, past the states a page may have skipped.
	tailSpan = time.Second
)

// updateTail sets the tail after the transcript grew from old to size.
func (h *Hosted) updateTail(f *transcript.File, old, size int64) {
	now := time.Now()
	h.mu.Lock()
	if h.tailAt.IsZero() || now.Sub(h.tailAt) > tailSpan || h.tailFrom > old {
		h.tailFrom, h.tailAt = old, now
	}
	from := h.tailFrom
	h.mu.Unlock()
	t := &api.BlockTail{Start: from, End: size}
	if size-from <= TailBytes {
		if p, err := f.After(from); err == nil && p.End == size && small(p.Blocks) {
			t.Blocks = p.Blocks
			// The files they name, as a page of messages has them (Manager.SetAnnotate).
			if a := h.m.annotate.Load(); a != nil {
				(*a)(h.id, h.meta.Cwd, t.Blocks)
			}
		}
	}
	h.mu.Lock()
	h.tail = t
	h.mu.Unlock()
}

// Annotate fills in blocks' Files (files.Service.AnnotateBlocks): what they name that exists.
type Annotate func(session, cwd string, blocks []api.Block)

// SetInChannel has f say whether a session's channel has other agents in it
// (docs/channels.md): the title line after a message is then the server's to give (titleNow).
func (m *Manager) SetInChannel(f func(id string) bool) { m.inChan.Store(&f) }

func (m *Manager) inChannel(id string) bool {
	if f := m.inChan.Load(); f != nil {
		return (*f)(id)
	}
	return false
}

// SetChannelOf has f say, of a session, the agents of its channel where it has several
// (session and nick) and the channels it is in beside its own; List marks the sessions with it.
func (m *Manager) SetChannelOf(f func(id string) (agents []api.ChannelAgent, in []string)) {
	m.chanOf.Store(&f)
}

// SetSpeaks has f say whether a session's agent is in a channel by a subscription
// (docs/channels.md): saying something there with ir is then not asked about (speaksOnly).
func (m *Manager) SetSpeaks(f func(session, channel string) bool) { m.speaksIn.Store(&f) }

func (m *Manager) speaks(session, channel string) bool {
	if f := m.speaksIn.Load(); f != nil {
		return (*f)(session, channel)
	}
	return false
}

// SetCleared has f told when a session is cleared: old, and the session that goes on in its
// place (Clear, clearedInside), before old goes to the archive.
func (m *Manager) SetCleared(f func(old, next string)) { m.cleared.Store(&f) }

func (m *Manager) tellCleared(old, next string) {
	if f := m.cleared.Load(); f != nil && old != next {
		(*f)(old, next)
	}
}

// SetSpawned has f say where a session was started, if for a channel; List marks them with it.
func (m *Manager) SetSpawned(f func(id string) *api.Spawned) { m.spawned.Store(&f) }

// markChannels writes each session's agents, the channels it is in and where it was started
// into list.
func (m *Manager) markChannels(list []api.Session) {
	if f := m.spawned.Load(); f != nil {
		for i := range list {
			list[i].Spawned = (*f)(list[i].ID)
		}
	}
	f := m.chanOf.Load()
	if f == nil {
		return
	}
	kind := map[string]api.Agent{}
	for _, s := range list {
		kind[s.ID] = s.Agent
	}
	for i := range list {
		agents, in := (*f)(list[i].ID)
		for j := range agents {
			agents[j].Agent = kind[agents[j].Session]
		}
		list[i].Agents, list[i].In = agents, in
	}
}

// SetAnnotate makes the pages of messages and the tails of the sessions' states carry the files
// their blocks name. The server sets it; it comes after the manager.
func (m *Manager) SetAnnotate(a Annotate) { m.annotate.Store(&a) }

// small: no block cut short, and little to carry.
func small(blocks []api.Block) bool {
	for _, b := range blocks {
		if b.Length > 0 || (b.Result != nil && b.Result.Length > 0) {
			return false
		}
	}
	j, err := json.Marshal(blocks)
	return err == nil && len(j) <= TailJSON
}

// ctxEvery: within a turn, the context is read again on a tool call this long after the last read.
const ctxEvery = 30 * time.Second

// refreshContext asks the agent how full its context is (Claude: get_context_usage): at the end of
// each turn, after a compaction, and within a turn on a tool call at most every ctxEvery. The answer is kept beside the holder's files, so it is there
// when no process runs.
func (h *Hosted) refreshContext() {
	ctx, cancel := context.WithTimeout(h.m.ctx, 10*time.Second)
	defer cancel()
	c, err := h.drv.Context(ctx)
	if err != nil {
		return
	}
	if err := writeJSON(filepath.Join(h.dir, fileContext), c); err != nil {
		h.logf("context: %v", err)
	}
	h.mu.Lock()
	h.ctxSum = &api.ContextSummary{Used: c.Used, Max: c.Max, At: c.At}
	h.mu.Unlock()
	h.m.o.Hub.Set("context:"+h.id, c)
	h.publish()
}

// savedContext is the last context read, nil if none.
func (h *Hosted) savedContext() *api.Context {
	b, err := os.ReadFile(filepath.Join(h.dir, fileContext))
	if err != nil {
		return nil
	}
	var c api.Context
	if json.Unmarshal(b, &c) != nil {
		return nil
	}
	return &c
}

// Context is the session's context as last read; nil if never.
func (m *Manager) Context(id string) (*api.Context, error) {
	h, err := m.get(id)
	if err != nil {
		return nil, err
	}
	return h.savedContext(), nil
}

func (h *Hosted) refreshQuota() {
	ctx, cancel := context.WithTimeout(h.m.ctx, 10*time.Second)
	defer cancel()
	if u, err := h.drv.Quota(ctx); err == nil && len(u.Windows) > 0 {
		h.setQuota(u.Windows, nil)
	}
	h.refreshBalance(ctx)
}

// refreshBalance reads the account's money, for a driver that has it (dsh); true if it did. Not
// for a session at another machine: its key and files are there, not here (docs/exec.md "可选").
func (h *Hosted) refreshBalance(ctx context.Context) bool {
	b, ok := h.drv.(agent.Balancer)
	if !ok || h.meta.Machine != "" {
		return false
	}
	got, err := b.Balance(ctx)
	if err != nil || len(got) == 0 {
		return false
	}
	h.setQuota(nil, got)
	return true
}

func (h *Hosted) setQuota(w []api.QuotaWindow, balance []api.Balance) {
	h.mu.Lock()
	account, plan := h.account, h.plan
	h.mu.Unlock()
	h.m.setQuota(api.AgentQuota{Agent: h.meta.Agent, Account: account, Machine: h.meta.Machine, Plan: plan, Windows: w, Balance: balance})
}

// quotaKey tells one login's windows from another's: the agent, the account it reports, and the
// machine whose login it is. The account alone does not: Codex reports none, and an executor's
// session runs under that machine's login (its ~/.codex/auth.json), maybe another account.
func quotaKey(a api.Agent, account, machine string) string {
	return string(a) + "\x00" + account + "\x00" + machine
}

func keyOf(q api.AgentQuota) string { return quotaKey(q.Agent, q.Account, q.Machine) }

// quotaKeyLocked is the key of the session's login; h.mu held.
func (h *Hosted) quotaKeyLocked() string { return quotaKey(h.meta.Agent, h.account, h.meta.Machine) }

func (m *Manager) setQuota(q api.AgentQuota) {
	q.At = time.Now().UnixMilli()
	m.mu.Lock()
	m.quota[keyOf(q)] = q
	saved := make([]api.AgentQuota, 0, len(m.quota))
	for _, q := range m.quota {
		saved = append(saved, q)
	}
	m.mu.Unlock()
	if err := writeJSON(filepath.Join(m.o.Config.StatePath(), fileQuota), saved); err != nil {
		m.o.Logf("session: quota: %v", err)
	}
}

func (m *Manager) loadQuota() {
	b, err := os.ReadFile(filepath.Join(m.o.Config.StatePath(), fileQuota))
	if err != nil {
		return
	}
	var saved []api.AgentQuota
	if json.Unmarshal(b, &saved) != nil {
		return
	}
	m.mu.Lock()
	for _, q := range saved {
		m.quota[keyOf(q)] = q
	}
	m.mu.Unlock()
}

// Quota is the windows and balances per agent, account and machine, as last read; live where a
// process of theirs runs. Asked for (GET /v1/internal/quota), not carried: the page asks again
// each minute (kumo, 2026-11-24).
func (m *Manager) Quota() api.Quota {
	live := map[string]bool{}
	for _, h := range m.all() {
		h.mu.Lock()
		if h.live {
			live[h.quotaKeyLocked()] = true
		}
		h.mu.Unlock()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	q := api.Quota{Agents: []api.AgentQuota{}}
	for k, a := range m.quota {
		a.Live = live[k]
		q.Agents = append(q.Agents, a)
	}
	slices.SortFunc(q.Agents, func(a, b api.AgentQuota) int {
		return strings.Compare(keyOf(a), keyOf(b))
	})
	return q
}

// QuotaOf is the windows of the session's agent, account and machine, nil if none were read.
func (m *Manager) QuotaOf(id string) (*api.AgentQuota, error) {
	h, err := m.get(id)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	k := h.quotaKeyLocked()
	h.mu.Unlock()
	for _, q := range m.Quota().Agents {
		if keyOf(q) == k {
			return &q, nil
		}
	}
	return nil, nil
}

// quotaLoop reads the windows again every QuotaEvery, through one running process of each agent,
// account and machine.
func (m *Manager) quotaLoop() {
	defer m.wg.Done()
	t := time.NewTicker(m.o.QuotaEvery)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
		}
		seen := map[string]bool{}
		for _, h := range m.all() {
			h.mu.Lock()
			k, ok := h.quotaKeyLocked(), h.live && !h.starting
			h.mu.Unlock()
			if ok && !seen[k] {
				seen[k] = true
				go h.refreshQuota()
			}
		}
	}
}

func (m *Manager) all() []*Hosted {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Hosted, 0, len(m.hosted))
	for _, h := range m.hosted {
		out = append(out, h)
	}
	return out
}

// SetCreated has f run on a session just made, before its agent starts: what was asked for
// with it (req: its role, its card). An error undoes nothing: the session is there without it.
func (m *Manager) SetCreated(f func(ctx context.Context, id string, req api.NewSession) error) {
	m.created.Store(&f)
}

// SetSelf has f say what a session's agent is told of itself in its system prompt
// (assemble.Self), at each start of its process.
func (m *Manager) SetSelf(f func(id string) *assemble.Self) { m.selfOf.Store(&f) }

func (m *Manager) self(id string) *assemble.Self {
	if f := m.selfOf.Load(); f != nil {
		return (*f)(id)
	}
	return nil
}
