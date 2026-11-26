package server

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kumoSleeping/iridium/internal/api"
)

// A call held for its tasks is waited out for three seconds in tests, not half a minute: long
// enough that a machine busy with every test at once does not let one go before its turn began.
func init() { heldGrace = 3 * time.Second }

// held are session id's calls held for background tasks.
func (l *live) held(id string) []called {
	l.s.calls.mu.Lock()
	defer l.s.calls.mu.Unlock()
	var out []called
	for _, c := range l.s.calls.load(id) {
		if c.Held {
			out = append(out, c)
		}
	}
	return out
}

// waitHeld waits until session id has n calls open, all held for its background tasks, and gives
// the first task they wait for.
func (l *live) waitHeld(id string, n int) string {
	l.t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		l.s.calls.mu.Lock()
		all := len(l.s.calls.load(id))
		l.s.calls.mu.Unlock()
		if h := l.held(id); len(h) == n && all == n {
			var st api.SessionState
			if l.do("web", "GET", "/v1/internal/sessions/"+id, nil, &st); st.Status == api.StatusIdle {
				if n == 0 || len(h[0].Tasks) == 0 {
					return ""
				}
				return h[0].Tasks[0]
			}
		}
		if time.Now().After(deadline) {
			l.t.Fatalf("%s: %d calls open, held %+v; want %d held", id, all, l.held(id), n)
		}
	}
}

// count is how many messages of channel id have part.
func (l *live) count(id, part string) (n int) {
	for _, m := range l.said("web", id) {
		if strings.Contains(m, part) {
			n++
		}
	}
	return n
}

func (l *live) stopTask(id, task string) {
	l.t.Helper()
	l.want(200, "web", "POST", "/v1/internal/sessions/"+id+"/tasks/"+task+"/stop", nil, nil)
}

// An agent called in a channel sends a task to the background and ends its turn; as the task
// ends it begins a turn itself, with no message of ours, and what it writes then is said in the
// channel too (kumo, 2026-11-26: the report stayed in the agent's own session, the call closed
// as the first turn ended). No line that it said nothing meanwhile; the call closed after.
func TestBackgroundReportSaidWhereCalled(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	code, r := l.send("web", ch, api.Send{Text: "@claude2 build it BGREPORT"})
	if code != 200 {
		t.Fatalf("send: %d", code)
	}
	l.waitSaid("web", ch, fmt.Sprintf("ok: #%d: @claude2 build it BGREPORT", r.Seq))
	l.waitSaid("web", ch, "report: brep1")
	l.noCalls(second)
	l.idle(second)
	time.Sleep(300 * time.Millisecond)
	if n := l.count(ch, "report: brep1"); n != 1 {
		t.Errorf("the report said %d times:\n%s", n, strings.Join(l.said("web", ch), "\n"))
	}
	if n := l.count(ch, "未在频道发话"); n != 0 {
		t.Errorf("told it said nothing:\n%s", strings.Join(l.said("web", ch), "\n"))
	}
}

// With no words in the turn that started the task, nothing is told of its silence while the
// task runs: the report is its answer.
func TestBackgroundSilentFirstTurn(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	if code, _ := l.send("web", ch, api.Send{Text: "@claude2 (say nothing) BGSTAY"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	task := l.waitHeld(second, 1)
	time.Sleep(heldGrace + 500*time.Millisecond)
	if n := l.count(ch, "未在频道发话"); n != 0 {
		t.Fatalf("told it said nothing while its task ran:\n%s", strings.Join(l.said("web", ch), "\n"))
	}
	if len(l.held(second)) != 1 {
		t.Fatal("the held call was closed while its task ran")
	}
	l.stopTask(second, task)
	l.waitSaid("web", ch, "report: "+task)
	l.noCalls(second)
	if n := l.count(ch, "未在频道发话"); n != 0 {
		t.Errorf("told it said nothing:\n%s", strings.Join(l.said("web", ch), "\n"))
	}
}

// While the task runs: another call of the same channel is answered and closed, the held one
// kept; what the agent does in its own session stays there; then the report is said where the
// task was asked for.
func TestBackgroundOtherTurnsMeanwhile(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	if code, _ := l.send("web", ch, api.Send{Text: "@claude2 build it BGSTAY"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	task := l.waitHeld(second, 1)

	code, r := l.send("web", ch, api.Send{Text: "@claude2 and this"})
	if code != 200 {
		t.Fatalf("send: %d", code)
	}
	l.waitSaid("web", ch, fmt.Sprintf("ok: #%d: @claude2 and this", r.Seq))
	if got := l.waitHeld(second, 1); got != task {
		t.Fatalf("held for %q, want %q", got, task)
	}

	if code, _ := l.send("web", second, api.Send{Text: "its own work"}); code != 200 {
		t.Fatalf("send to the second: %d", code)
	}
	l.waitReply("web", second, "ok: its own work")
	l.waitHeld(second, 1)
	time.Sleep(300 * time.Millisecond)
	if n := l.count(ch, "its own work"); n != 0 {
		t.Fatalf("its own session's turn said in the channel:\n%s", strings.Join(l.said("web", ch), "\n"))
	}

	l.stopTask(second, task)
	l.waitSaid("web", ch, "report: "+task)
	l.noCalls(second)
	if n := l.count(ch, "未在频道发话"); n != 0 {
		t.Errorf("told it said nothing:\n%s", strings.Join(l.said("web", ch), "\n"))
	}
}

// An agent in two channels, a task running for each: each report is said in the channel its
// task was asked for in, not in the other.
func TestBackgroundReportsAcrossChannels(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	other := l.create(200, "web", api.NewSession{Agent: api.AgentClaude, Cwd: l.work, Text: "another"})
	l.waitReply("web", other, "ok: another")
	l.want(200, "web", "POST", "/v1/internal/channels/"+other+"/members", api.NewMember{Session: second}, nil)
	nick := l.s.ch.Nick(second)

	if code, _ := l.send("web", ch, api.Send{Text: "@" + nick + " one BGSTAY"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	first := l.waitHeld(second, 1)
	if code, _ := l.send("web", other, api.Send{Text: "@" + nick + " two BGSTAY"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	l.waitHeld(second, 2)
	var later string
	for _, c := range l.held(second) {
		if c.Channel == other {
			if len(c.Tasks) != 1 || c.Tasks[0] == first {
				t.Fatalf("the other channel's call waits for %v (the first: %s)", c.Tasks, first)
			}
			later = c.Tasks[0]
		} else if !slices.Equal(c.Tasks, []string{first}) {
			t.Fatalf("the first channel's call waits for %v, want %s", c.Tasks, first)
		}
	}

	// The earlier one ends first: its report goes to the earlier channel, though the other
	// call is the later.
	l.stopTask(second, first)
	l.waitSaid("web", ch, "report: "+first)
	l.waitHeld(second, 1)
	l.stopTask(second, later)
	l.waitSaid("web", other, "report: "+later)
	l.noCalls(second)
	time.Sleep(300 * time.Millisecond)
	if l.count(other, "report: "+first) != 0 || l.count(ch, "report: "+later) != 0 {
		t.Errorf("a report said in the wrong channel:\n%s\n--\n%s", strings.Join(l.said("web", ch), "\n"), strings.Join(l.said("web", other), "\n"))
	}
	for _, c := range []string{ch, other} {
		if n := l.count(c, "未在频道发话"); n != 0 {
			t.Errorf("told it said nothing:\n%s", strings.Join(l.said("web", c), "\n"))
		}
	}
}

// A task stopped, with no turn of the agent's after: the held call is closed once it was waited
// out, and not before. It had spoken: no line.
func TestBackgroundTaskStoppedNoReport(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	if code, _ := l.send("web", ch, api.Send{Text: "@claude2 build it BGQUIET"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	task := l.waitHeld(second, 1)
	l.stopTask(second, task)
	time.Sleep(heldGrace / 2)
	if len(l.held(second)) != 1 {
		t.Fatal("closed before it was waited out")
	}
	l.noCalls(second)
	if n := l.count(ch, "未在频道发话"); n != 0 {
		t.Errorf("told it said nothing:\n%s", strings.Join(l.said("web", ch), "\n"))
	}
}

// The agent's process stopped with a task running: the task went with it, the held call is
// closed.
func TestBackgroundProcessStopped(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	if code, _ := l.send("web", ch, api.Send{Text: "@claude2 build it BGQUIET"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	l.waitHeld(second, 1)
	l.want(200, "web", "POST", "/v1/internal/sessions/"+second+"/stop", nil, nil)
	l.noCalls(second)
}

// No call open and a turn the agent began itself: what it writes is said in the channel it was
// started for, the only one it is in.
func TestOwnTurnSaidInItsChannel(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	if code, _ := l.send("web", ch, api.Send{Text: "@claude2 build it BGSTAY"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	task := l.waitHeld(second, 1)
	l.s.calls.take(second, func(called) bool { return true })
	l.stopTask(second, task)
	l.waitSaid("web", ch, "report: "+task)
	l.idle(second)
	time.Sleep(300 * time.Millisecond)
	if n := l.count(ch, "report: "+task); n != 1 {
		t.Errorf("said %d times:\n%s", n, strings.Join(l.said("web", ch), "\n"))
	}
}

// A call held as the daemon stopped. Its task over by the time the next one looks, the agent at
// rest: what it wrote meanwhile is said where it was called and the call closed, as any turn
// that ended while nobody listened. Its task running still: it stays held.
func TestHeldCallAcrossRestart(t *testing.T) {
	t.Parallel()
	l, ch, second := two(t)
	if code, _ := l.send("web", second, api.Send{Text: "the report"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	l.waitReply("web", second, "ok: the report")
	l.idle(second)
	// That turn's end done with, as it was before the daemon stopped.
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, self := l.s.answers.given(second); self {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("its turn's end was not told")
		}
	}
	done := make(chan struct{})
	l.s.answers.add(second, func() { close(done) })
	<-done
	l.s.calls.add(second, called{Channel: ch, Seq: 1, Spoke: true, Held: true, Tasks: []string{"gone"}})
	l.s.settleMissed()
	l.waitSaid("web", ch, "ok: the report")
	l.noCalls(second)

	if code, _ := l.send("web", ch, api.Send{Text: "@claude2 build it BGSTAY"}); code != 200 {
		t.Fatalf("send: %d", code)
	}
	task := l.waitHeld(second, 1)
	l.s.settleMissed()
	time.Sleep(300 * time.Millisecond)
	if got := l.waitHeld(second, 1); got != task {
		t.Fatalf("held for %q, want %q", got, task)
	}
	l.stopTask(second, task)
	l.waitSaid("web", ch, "report: "+task)
	l.noCalls(second)
}

// calls.settle and calls.answering, turn by turn.
func TestCallsHeld(t *testing.T) {
	t.Parallel()
	all := func(called) bool { return true }
	run := func(ts ...string) func() []string { return func() []string { return ts } }
	k := &calls{}
	k.add("s", called{Channel: "a", Seq: 1, Input: "i1", Spoke: true})

	// A turn that took it ends with a task running: held, nothing over.
	over, wait := k.settle("s", all, []string{"t1"}, false, false)
	if len(over) != 0 || wait || !slices.Equal(k.of["s"][0].Tasks, []string{"t1"}) || !k.of["s"][0].Held {
		t.Fatalf("held: over %v wait %v calls %+v", over, wait, k.of["s"])
	}
	// A turn that took an input answers no held call; one of the agent's own does.
	if c, ok := k.answering("s", nil, false, run("t1")); ok {
		t.Fatalf("a turn with an input answers the held call %+v", c)
	}
	if c, ok := k.answering("s", nil, true, run()); !ok || c.Channel != "a" {
		t.Fatalf("its own turn answers %+v %v", c, ok)
	}
	// Another call taken meanwhile, its turn with a task of its own.
	k.add("s", called{Channel: "b", Seq: 2, Input: "i2"})
	if c, ok := k.answering("s", []string{"i2"}, false, run("t1")); !ok || c.Channel != "b" {
		t.Fatalf("the turn that took the second answers %+v %v", c, ok)
	}
	over, wait = k.settle("s", all, []string{"t1", "t2"}, false, false)
	if len(over) != 0 || wait || !slices.Equal(k.of["s"][1].Tasks, []string{"t2"}) || !slices.Equal(k.of["s"][0].Tasks, []string{"t1"}) {
		t.Fatalf("two held: over %v calls %+v", over, k.of["s"])
	}
	// A third, with no task of its own: over as its turn ends, the held ones kept.
	k.add("s", called{Channel: "b", Seq: 3, Input: "i3"})
	over, _ = k.settle("s", all, []string{"t1", "t2"}, false, false)
	if len(over) != 1 || over[0].Seq != 3 || len(k.of["s"]) != 2 {
		t.Fatalf("the third: over %+v calls %+v", over, k.of["s"])
	}
	// The first's task over: the agent's own turn answers the first, not the later held one.
	if c, ok := k.answering("s", nil, true, run("t2")); !ok || c.Channel != "a" {
		t.Fatalf("with t1 over its own turn answers %+v %v", c, ok)
	}
	// That turn starts a task of its own: the call it answered waits for it.
	over, wait = k.settle("s", all, []string{"t2", "t3"}, true, false)
	if len(over) != 0 || wait || !slices.Equal(k.of["s"][0].Tasks, []string{"t3"}) {
		t.Fatalf("a task begun in its own turn: over %v calls %+v", over, k.of["s"])
	}
	// t2 over within a turn that took an input: its call is waited out, not closed.
	over, wait = k.settle("s", all, []string{"t3"}, false, false)
	if len(over) != 0 || !wait || len(k.of["s"]) != 2 || !k.of["s"][1].idle {
		t.Fatalf("over in another turn: over %v wait %v calls %+v", over, wait, k.of["s"])
	}
	// Known already: not due again.
	if k.lapsed("s", []string{"t3"}) {
		t.Fatal("due twice")
	}
	// The agent's own turn ends: the one with no task left is over, the other kept.
	over, _ = k.settle("s", all, []string{"t3"}, true, false)
	if len(over) != 1 || over[0].Channel != "b" || len(k.of["s"]) != 1 {
		t.Fatalf("its own turn's end: over %+v calls %+v", over, k.of["s"])
	}
	// t3 over while it rests: due once.
	if !k.lapsed("s", nil) || k.lapsed("s", nil) {
		t.Fatal("a held call with no task left is due once")
	}
	over, _ = k.settle("s", all, nil, true, false)
	if len(over) != 1 || len(k.of["s"]) != 0 {
		t.Fatalf("the last: over %+v calls %+v", over, k.of["s"])
	}

	// Tasks ran as the turn ended and none does by the time it is looked at: held all the same,
	// waited out; the agent's own turn then answers it and closes it.
	k.add("s", called{Channel: "a", Seq: 4, Input: "i4"})
	over, wait = k.settle("s", all, nil, false, true)
	if len(over) != 0 || !wait || !k.of["s"][0].Held {
		t.Fatalf("late: over %v wait %v calls %+v", over, wait, k.of["s"])
	}
	if c, ok := k.answering("s", nil, true, run()); !ok || c.Seq != 4 {
		t.Fatalf("late: its own turn answers %+v %v", c, ok)
	}
	if over, _ = k.settle("s", all, nil, true, false); len(over) != 1 {
		t.Fatalf("late: over %+v", over)
	}
	// A call not yet taken is no turn's: kept, never held.
	k.add("s", called{Channel: "a", Seq: 5, Input: "i5"})
	over, _ = k.settle("s", func(called) bool { return false }, []string{"t9"}, false, false)
	if len(over) != 0 || k.of["s"][0].Held {
		t.Fatalf("not taken: over %v calls %+v", over, k.of["s"])
	}
}
