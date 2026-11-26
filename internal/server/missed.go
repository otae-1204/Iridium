package server

import (
	"context"
	"slices"
	"time"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/channel"
	"github.com/kumoSleeping/iridium/internal/store"
)

// A turn that ended while no daemon listened (kumo, 2026-11-25: an update as an agent wrote its
// last words): the output it was read from is replayed into the record with no notice, so its
// end closed no call and its last words, read into its own channel as any others, were said
// nowhere else. The agent had answered; the channel it was called in never had it.
//
// Once the sessions are taken up, each one at rest that still has a call from before this process
// (calls.open: old) has the last thing it said since it was called said there as its reply, unless
// the channel has those words of it already, and the call closed.

// missedAfter is how long after it starts the daemon looks for them: the sessions taken up, what
// they wrote meanwhile read.
var missedAfter = 10 * time.Second

func (s *Server) missedLoop(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(missedAfter):
	}
	s.settleMissed()
}

// olds are the sessions with a call kept from before this process, or one held for background
// tasks (called.Held), which may have ended meanwhile.
func (k *calls) olds() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for id, cs := range k.of {
		for _, c := range cs {
			if c.old || c.Held {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

// stale makes session's held calls with none of their tasks among running old ones: kept from
// before this process, their tasks over while nobody listened, and so the turn that told of them.
func (k *calls) stale(session string, running []string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	cs := k.load(session)
	for i := range cs {
		c := &cs[i]
		if c.Held && !c.idle && !slices.ContainsFunc(c.Tasks, func(t string) bool { return slices.Contains(running, t) }) {
			c.Held, c.Tasks, c.old = false, nil, true
		}
	}
	k.save(session)
}

func (s *Server) settleMissed() {
	for _, id := range s.calls.olds() {
		st, err := s.m.State(id)
		// At work still: its turn goes on, and says its words where it was called as it writes
		// them (calls.answering).
		if err != nil || st.Status == api.StatusRunning || st.Status == api.StatusCompacting || st.Status == api.StatusStarting {
			continue
		}
		// Held for tasks that are over: as any call whose turn ended meanwhile. With one running
		// still, it stays held for the turn that reports it.
		s.calls.stale(id, s.tasksOf(id))
		s.ch.Touch(id) // what it wrote meanwhile, into its own channel
		s.answers.add(id, func() { s.missed(id) })
	}
}

// missed says session id's missed reply where it was called, and closes its old calls; in its
// turn of answers.
func (s *Server) missed(id string) {
	unlock := s.chain.lock(id)
	c, ok := s.calls.answering(id, nil, false, nil)
	unlock()
	if ok && c.old && c.Channel != id && s.ch.Speaks(c.Channel, id) {
		if m, ok := s.lastSaidSince(id, c.From); ok && !s.saidThere(c.Channel, id, m.Text) {
			s.logf("server: channel %s: %s's reply to #%d, written while nobody listened, said now", c.Channel, id, c.Seq)
			// As a reply heard live is (routeSaid): said there, given to whom it names. Queued
			// after this, before the calls close.
			s.routeSaid(s.ch.Final(m))
		}
	}
	s.answers.add(id, func() {
		unlock := s.chain.lock(id)
		over := s.calls.take(id, func(c called) bool { return c.old })
		unlock()
		s.closeCalls(id, over)
	})
}

// lastSaidSince is the last thing session's agent said in its own channel, of its record from
// line from on.
func (s *Server) lastSaidSince(session string, from int64) (channel.Message, bool) {
	rows, err := s.ch.List(session, 0, -1, 30)
	if err != nil {
		return channel.Message{}, false
	}
	me := store.AgentMember(session)
	for i := len(rows) - 1; i >= 0; i-- {
		m := rows[i]
		if m.Gone || m.Kind != channel.KindSaid || m.Author != me || m.D.Sent || m.D.Block == "" {
			continue
		}
		if line(m.D.Block) < from {
			break
		}
		return m, true
	}
	return channel.Message{}, false
}

// saidThere: channel ch has text as words of session's agent among its latest.
func (s *Server) saidThere(ch, session, text string) bool {
	rows, err := s.ch.List(ch, 0, -1, 30)
	if err != nil {
		return true // not known: not said twice
	}
	me := store.AgentMember(session)
	for _, m := range rows {
		if !m.Gone && m.Kind == channel.KindSaid && m.Author == me && channel.Same(m.Text, text) {
			return true
		}
	}
	return false
}
