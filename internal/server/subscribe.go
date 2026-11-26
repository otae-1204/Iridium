package server

// The subscriptions of a channel's members (docs/channels.md "订阅事件"): what wakes an agent
// beside what names it, what runs a script. Every member of a channel is given what is said there
// (it reads it as it is next woken); its subscriptions say when Iridium wakes it or runs it.

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/channel"
	"github.com/kumoSleeping/iridium/internal/cron"
	"github.com/kumoSleeping/iridium/internal/deliver"
	"github.com/kumoSleeping/iridium/internal/gate"
	"github.com/kumoSleeping/iridium/internal/satori"
	"github.com/kumoSleeping/iridium/internal/session"
	"github.com/kumoSleeping/iridium/internal/store"
)

// makerOn is what a new session's own agent is subscribed to: the words of whoever made it
// (kumo, 2026-11-23). Made at the panel or the host: of all the panel's logins and the host's
// (one person, at any of their browsers); by another app's login: of that login. An agent that
// makes one acts for its turn's login.
func makerOn(c caller) []api.Trigger {
	from := []string{readerOf(c)}
	if c.Host() || c.App == api.WebApp || c.App == "" {
		from = slices.Clone(DefaultPeople)
	}
	return []api.Trigger{{Message: &api.MessageTrigger{From: from}}}
}

// wakesOwn: what c says in session id's own channel, where it is the only agent, wakes it: it
// names it, or it is for one of its subscriptions; texts, the words of those that are.
func (s *Server) wakesOwn(id string, c caller, text string) (bool, []api.Trigger) {
	m, ok := s.memberRow(id, store.AgentMember(id))
	if !ok {
		return true, nil
	}
	a := deliver.Agent{Session: id, Nick: s.ch.Nick(id), On: m.On()}
	d := deliver.Route(satori.AtWords(text, s.ch.Nick), deliver.From{Member: readerOf(c)}, []deliver.Agent{a})
	return len(d.Wake) > 0, d.Texts[id]
}

// heldWords are what was said in session id's own channel since its agent was last woken there
// and was not given to it (channel.Data.Held), as lines before the words that wake it now.
func (s *Server) heldWords(id string) string {
	ms, err := s.ch.List(id, 0, -1, 30)
	if err != nil {
		return ""
	}
	var held []string
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		if m.Kind != channel.KindUser {
			continue // the agent's own words, Iridium's lines: not what it was not given
		}
		if !m.D.Held {
			break
		}
		held = append(held, fmt.Sprintf("#%d %s: %s", m.Seq, cmpOr(m.Name, m.Author), satori.ForwardWords(m.Text)))
	}
	if len(held) == 0 {
		return ""
	}
	slices.Reverse(held)
	return "Said here since you were last woken, not given to you then:\n" + strings.Join(held, "\n") + "\n\nNow:\n"
}

// onText is what the entries of an agent's on that woke it ask of it, a line each after what woke
// it: "[woken: a member joins] greet them".
func (s *Server) onText(ts []api.Trigger) string {
	var b strings.Builder
	for i, t := range ts {
		if i == 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "\n[woken: %s] %s", s.ruleWords(t), t.Text)
	}
	return b.String()
}

// eventKinds are the kinds of event (api.Events) a line of a channel, or an agent's error or ask
// (ev.Type), is: "people" a join where people came or everyone may speak now, a leave where
// people went; the kinds before the subscriptions as theirs now (channel.EventKind).
func eventKinds(ev *channel.Event) []string {
	if ev == nil {
		return nil
	}
	if ev.Type == "people" {
		var out []string
		if len(ev.Added) > 0 || ev.Everyone {
			out = append(out, "join")
		}
		if len(ev.Removed) > 0 {
			out = append(out, "leave")
		}
		return out
	}
	if k := channel.EventKind(ev.Type); slices.Contains(api.Events, k) {
		return []string{k}
	}
	return nil
}

// eventFires: one of on is for an event of kinds; texts, those of them with words.
func eventFires(on []api.Trigger, kinds []string) (texts []api.Trigger, ok bool) {
	for _, t := range on {
		if slices.ContainsFunc(t.Event, func(e string) bool { return slices.Contains(kinds, channel.EventKind(e)) }) {
			ok = true
			if t.Text != "" {
				texts = append(texts, t)
			}
		}
	}
	return texts, ok
}

// eventWakes: a line of channel ch (number seq, words text), or an agent's error or ask there,
// told of ev: the agents subscribed to an event of its kinds are woken, not the one it is about.
// What an agent's doing set off goes on its run, and counts toward the agents in a row.
func (s *Server) eventWakes(ch string, seq int64, ev *channel.Event, text string, kinds []string) {
	list, _ := s.agentsOf(ch)
	who, lk := deliver.From{Member: satori.NoteUser}, s.chain.of(ch, "")
	if id, ok := store.SessionOfMember(ev.Member); ok {
		who, lk = deliver.From{Member: ev.Member, Session: id}, s.chain.of(ch, id)
	}
	stopped := false
	for _, a := range list {
		if store.AgentMember(a.Session) == ev.Member {
			continue
		}
		texts, ok := eventFires(a.On, kinds)
		if !ok {
			continue
		}
		if passes(who) && lk.hop >= deliver.MaxHops {
			if !stopped {
				s.stopped(ch, seq, deliver.MaxHops, lk.origin)
				stopped = true
			}
			continue
		}
		go s.wakeFor(ch, a.Session, seq, text+s.onText(texts), s.chain.reach(who, lk))
	}
}

// wakeFor gives words to agent to in channel ch, as message seq of it: Iridium's, for a
// subscription.
func (s *Server) wakeFor(ch, to string, seq int64, words string, lk link) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := s.deliverTo(ctx, ch, to, seq, words, "", session.From(gate.HostID, "Iridium"), nil, lk); err != nil {
		s.logf("server: channel %s: a subscription of %s: %v", ch, to, err)
	}
}

// cronAgents wakes the agents a time of whose subscriptions is minute at, each once a minute:
// a line of the channel says so, and it is what they are given.
func (s *Server) cronAgents(at time.Time) {
	r := &s.scripts
	for _, m := range s.ch.Subscribers() {
		var texts []api.Trigger
		due := false
		for _, t := range m.On() {
			if t.Cron == "" {
				continue
			}
			if sp, err := cron.Parse(t.Cron); err == nil && sp.Match(at) {
				due = true
				if t.Text != "" {
					texts = append(texts, t)
				}
			}
		}
		if !due || !s.counts(m.Channel, m.Session) {
			continue
		}
		k := m.Channel + "\x00" + m.Member
		r.mu.Lock()
		if r.cron == nil {
			r.cron = map[string]time.Time{}
		}
		done := r.cron[k].Equal(at)
		r.cron[k] = at
		r.mu.Unlock()
		if done {
			continue
		}
		if st, err := s.m.State(m.Channel); err != nil || st.Archived {
			continue
		}
		text := fmt.Sprintf("定时 %s → @%s", at.Format("15:04"), m.Nick)
		seq := s.sat.Note(m.Channel, text, satori.NoteUser, &channel.Event{Type: "cron", Member: m.Member, Nick: m.Nick})
		if seq == 0 {
			continue
		}
		go s.wakeFor(m.Channel, m.Session, seq, text+s.onText(texts), s.chain.person(m.Channel, satori.NoteUser))
	}
}

// ring refuses subscriptions on for agent member of channel ch that would close a ring of agents
// alone (kumo, 2026-11-24: not allowed from the start, not warned of): each one's last words
// waking the next, round to the first, no one having called anyone. An agent calls another by
// naming it (@). A ring through a command is not this: its words name whom they wake (loops).
// nil: none, or on is not a change of them (unknown channel).
func (s *Server) ring(ch, member string, on []api.Trigger) error {
	ms, err := s.ch.Members(ch)
	if err != nil {
		return nil
	}
	type node struct {
		member, nick string
		on           []api.Trigger
	}
	var nodes []node
	first := -1
	for _, m := range ms {
		if !m.Agent() {
			continue
		}
		n := node{member: m.Member, nick: m.Nick, on: m.On()}
		if m.Member == member {
			n.on, first = on, len(nodes)
		}
		nodes = append(nodes, n)
	}
	if first < 0 {
		// Not of the channel yet (it joins with these): what it would be there.
		first = len(nodes)
		nodes = append(nodes, node{member: member, nick: "it", on: on})
	}
	wakes := func(a, b node) bool {
		return a.member != b.member && slices.ContainsFunc(b.on, func(t api.Trigger) bool {
			return t.Message != nil && (len(t.Message.From) == 0 || deliver.Receives(t.Message.From, a.member))
		})
	}
	seen := map[int]bool{}
	var path []int
	var walk func(i int) bool
	walk = func(i int) bool {
		path = append(path, i)
		seen[i] = true
		for j := range nodes {
			if !wakes(nodes[i], nodes[j]) {
				continue
			}
			if j == first || (!seen[j] && walk(j)) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}
	if !walk(first) {
		return nil
	}
	var names []string
	for _, k := range path {
		names = append(names, "@"+nodes[k].nick)
	}
	return fmt.Errorf("%w: a loop of agents waking each other by their last words alone: %s → %s; an agent calls another by naming it (@nick)", channel.ErrOn, strings.Join(names, " → "), names[0])
}

// loops are the rings of channel ch's subscriptions (warnings, a line each), those through
// member first: an agent's last words that wake another whose words wake the first, a script
// that names an agent whose words run it… Each goes round till MaxHops in a row.
func (s *Server) loops(ch, member string) []string {
	ms, err := s.ch.Members(ch)
	if err != nil {
		return nil
	}
	type node struct {
		member, name string
		on           []api.Trigger
		run          string // a script's command line
	}
	var nodes []node
	var nicks []string
	for _, m := range ms {
		if m.Agent() {
			nodes = append(nodes, node{member: m.Member, name: "@" + m.Nick, on: m.On()})
			nicks = append(nicks, m.Nick)
		}
	}
	if xs, err := s.st.Scripts(ch); err == nil {
		for _, x := range xs {
			if x.Enabled {
				nodes = append(nodes, node{member: scriptMember(x.Name), name: "「" + x.Name + "」", on: triggersOf(x), run: x.Run})
			}
		}
	}
	// wakes: what a's words set off of b. A script's words wake only whom they name; an agent's
	// last words, whoever's message subscription they are for.
	wakes := func(a, b node) bool {
		if a.member == b.member {
			return false
		}
		if a.run != "" {
			return b.run == "" && slices.ContainsFunc(deliver.Mentions(a.run, nicks), func(n string) bool { return "@"+n == b.name })
		}
		return slices.ContainsFunc(b.on, func(t api.Trigger) bool {
			return t.Message != nil && (len(t.Message.From) == 0 || deliver.Receives(t.Message.From, a.member))
		})
	}
	// The first ring from each node, as the names it goes through.
	var out []string
	seen := map[string]bool{}
	var path []int
	on := map[int]bool{}
	var walk func(i int) bool
	walk = func(i int) bool {
		path = append(path, i)
		on[i] = true
		defer func() { path = path[:len(path)-1]; on[i] = false }()
		for j := range nodes {
			if !wakes(nodes[i], nodes[j]) {
				continue
			}
			if j == path[0] {
				var names []string
				for _, k := range path {
					names = append(names, nodes[k].name)
				}
				key := strings.Join(slices.Sorted(slices.Values(names)), " ")
				if !seen[key] {
					seen[key] = true
					out = append(out, fmt.Sprintf("loop: %s → %s (stops after %d in a row)", strings.Join(names, " → "), nodes[path[0]].name, deliver.MaxHops))
				}
				return true
			}
			if !on[j] && j > path[0] && walk(j) {
				return true
			}
		}
		return false
	}
	for i := range nodes {
		walk(i)
	}
	// Those through member first.
	slices.SortStableFunc(out, func(a, b string) int {
		ia := strings.Contains(a, s.memberName(ch, member))
		ib := strings.Contains(b, s.memberName(ch, member))
		switch {
		case ia && !ib:
			return -1
		case ib && !ia:
			return 1
		}
		return 0
	})
	return out
}

// memberName is member as a ring names it: "@nick", a script's 「name」.
func (s *Server) memberName(ch, member string) string {
	if name, ok := strings.CutPrefix(member, scriptMemberPrefix); ok {
		return "「" + name + "」"
	}
	if id, ok := store.SessionOfMember(member); ok {
		return "@" + s.ch.Nick(id)
	}
	return member
}
