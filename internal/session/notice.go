package session

import "github.com/kumoSleeping/iridium/internal/api"

// NoticeKind is what happened in a session, for those that pass it on: plugins, bots.
type NoticeKind string

const (
	NoticeTurnStart NoticeKind = "turn_start"
	NoticeMessage   NoticeKind = "message"  // the agent said Text mid-turn (the last one is the reply)
	NoticeTool      NoticeKind = "tool"     // the agent calls Tool
	NoticeTurnEnd   NoticeKind = "turn_end" // Text: its reply; Background: tasks still run
	NoticeAsk       NoticeKind = "ask"      // Ask opened: needs you
	NoticeAskDone   NoticeKind = "ask_done" // RequestID answered or withdrawn
	NoticeError     NoticeKind = "error"    // the agent process ended badly: Text says how
	NoticeStatus    NoticeKind = "status"   // the lamp changed to Status
	NoticeCompacted NoticeKind = "compacted"
	// NoticeTaken: input Text is given to the agent now, told before it is written: what the
	// agent says after this notice may answer it, what it said before may not.
	NoticeTaken NoticeKind = "taken"
	// NoticeEnded: the agent process ended while a turn ran and the turn is not run again (a stop,
	// a crash not continued): no turn_end follows. Waiting as at a turn's end.
	NoticeEnded NoticeKind = "ended"
	// NoticeModel: the model the session runs as became known or changed (its agent said it, or
	// one was picked): what follows its model (its face) is looked at again.
	NoticeModel NoticeKind = "model"
	// NoticeWriting: the agent is writing a message, Text what came of it now (a piece; the
	// whole follows as NoticeMessage). Many a second: who watches must take it lightly.
	NoticeWriting NoticeKind = "writing"
	// NoticeUserRecorded: a user message was actually recorded into the agent's transcript at
	// byte range [Before, After). Unlike NoticeTaken (fired before Send), this marks the exact
	// moment the agent accepted the input into its conversation (matching TranscriptView).
	NoticeUserRecorded NoticeKind = "user_recorded"
	// NoticeTasks: the agent's background tasks changed (one began, one ended; none are left once
	// its process is gone): the session's state has them.
	NoticeTasks NoticeKind = "tasks"
)

// Notice is one thing that happened in a session, live (never from a replay).
type Notice struct {
	Session    string
	Kind       NoticeKind
	Text       string
	Tool       string
	Ask        *api.Ask
	RequestID  string
	Status     api.Status
	Background bool
	// Before, After on NoticeUserRecorded: byte offsets [Before, After) of the user block in the
	// session's transcript, used to bound the settled prior work and start the next carrier's work.
	Before, After int64
	// Asked: a compaction you asked for (/compact), not the agent's own as its context ran full.
	Asked bool
	// Denied: at a turn's end, how many of its calls auto mode denied.
	Denied int
	// Waiting, at a turn's end: the inputs that still wait in the queue, which the turn did not
	// take (what was given before it ended and is not among them, it took).
	Waiting []string
	// From, on a turn's start and an ask: the terminal that began the turn; Via, the session whose
	// agent sent the message that began it (ir send, ir new), if one did.
	From, Via string
}

// Watch calls f with every notice from now on, in order per session. f runs on the session's
// reading goroutine: it must not block for long, nor call back into a session synchronously.
func (m *Manager) Watch(f func(Notice)) {
	m.wmu.Lock()
	m.watchers = append(m.watchers, f)
	m.wmu.Unlock()
}

func (m *Manager) notify(ns ...Notice) {
	if len(ns) == 0 {
		return
	}
	m.wmu.Lock()
	ws := m.watchers
	m.wmu.Unlock()
	for _, n := range ns {
		for _, f := range ws {
			f(n)
		}
	}
}
