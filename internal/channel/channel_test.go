package channel

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/store"
)

// records are sessions with a record of one block a line, offsets 10 apart, read 3 at a time.
type records struct {
	mu     sync.Mutex
	blocks map[string][]api.Block
}

func (r *records) add(id string, b api.Block) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b.At = fmt.Sprintf("%d.0", len(r.blocks[id])*10)
	r.blocks[id] = append(r.blocks[id], b)
}

func (r *records) State(id string) (*api.SessionState, error) {
	if _, ok := r.blocks[id]; !ok {
		return nil, errors.New("no such session")
	}
	return &api.SessionState{Session: api.Session{ID: id}}, nil
}

func (r *records) Messages(id string, before, after int64) (api.MessagePage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bs := r.blocks[id]
	lo := min(int(max(after, 0)+9)/10, len(bs))
	hi := min(lo+3, len(bs))
	return api.MessagePage{Blocks: append([]api.Block{}, bs[lo:hi]...), Start: after, End: int64(hi * 10)}, nil
}

func (r *records) Full(id, at string) (string, error) { return "", errors.New("gone") }

func rig(t *testing.T) (*Service, *records, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "iridium.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r := &records{blocks: map[string][]api.Block{}}
	s := New(st, r, t.Logf)
	s.Numbered = true // the nicks as the tests read them (claude2); TestDefaultNick has the others
	return s, r, st
}

func texts(ms []Message) string {
	out := ""
	for _, m := range ms {
		out += fmt.Sprintf("%d:%s:%s ", m.Seq, m.Kind, m.Text)
	}
	return out
}

// A session's record is read into its channel once, in order: what people and the agent said,
// not its thinking or its tools; what comes later is added after, nothing twice.
func TestSync(t *testing.T) {
	s, r, _ := rig(t)
	r.add("s1", api.Block{Kind: api.BlockKindUser, Text: "hi", ID: "in1", From: "web", FromTerminal: "t1"})
	r.add("s1", api.Block{Kind: api.BlockKindThinking, Text: "hm"})
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "hello"})
	r.add("s1", api.Block{Kind: api.BlockKindTool, Name: "Bash"})
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "done"})
	r.add("s1", api.Block{Kind: api.BlockKindUser, Text: "typed in the terminal"})
	ms, err := s.List("s1", 0, -1, 0)
	if err != nil || texts(ms) != "1:user:hi 2:said:hello 3:said:done 4:user:typed in the terminal " {
		t.Fatalf("read in: %v %s", err, texts(ms))
	}
	if ms[0].Author != "t1" || ms[0].Name != "web" || ms[1].Author != "agent:s1" || ms[1].D.Block != "20.0" || ms[3].Author != HostTerminal {
		t.Errorf("who: %+v", ms)
	}
	// Again: nothing new. More in the record: after what is there.
	if added, err := s.Sync("s1"); err != nil || len(added) != 0 {
		t.Errorf("again: %v %s", err, texts(added))
	}
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "more"})
	if added, err := s.Sync("s1"); err != nil || texts(added) != "5:said:more " {
		t.Errorf("more: %v %s", err, texts(added))
	}
	// Pages: before a number, after one.
	if ms, _ := s.List("s1", 3, -1, 0); texts(ms) != "1:user:hi 2:said:hello " {
		t.Errorf("before 3: %s", texts(ms))
	}
	if ms, _ := s.List("s1", 0, 3, 1); texts(ms) != "4:user:typed in the terminal " {
		t.Errorf("after 3: %s", texts(ms))
	}
}

// What a person sends is said as it is sent, and not again when the agent's record has it; what
// the agent says before its record has it is said from its words, and is that block once found.
func TestSayAndHeard(t *testing.T) {
	s, r, _ := rig(t)
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "old", Time: 1})
	m, isNew, err := s.Say(Message{ChannelMessage: store.ChannelMessage{Channel: "s1", Author: "u1", Kind: KindUser, Text: "q", Ref: "in1"}})
	if err != nil || !isNew || m.Seq != 2 {
		t.Fatalf("said: %v %v %+v", err, isNew, m)
	}
	if _, isNew, _ := s.Say(Message{ChannelMessage: store.ChannelMessage{Channel: "s1", Author: "u1", Kind: KindUser, Text: "q", Ref: "in1"}}); isNew {
		t.Error("said twice")
	}
	r.add("s1", api.Block{Kind: api.BlockKindUser, Text: "q", ID: "in1", From: "web", FromTerminal: "t1"})
	// The agent speaks; its record lags.
	others, said, err := s.Heard("s1", "answer", 0, 0)
	if err != nil || len(others) != 0 || said.Seq != 3 || said.Ref != "" {
		t.Fatalf("heard live: %v %s %+v", err, texts(others), said)
	}
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "answer", Files: []api.FileRef{{Ref: "f1"}}})
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "next"})
	others, said, err = s.Heard("s1", "next", 3, 0)
	if err != nil || len(others) != 0 || said.Seq != 4 || said.Ref != "b:s1:30.0" {
		t.Fatalf("heard from the record: %v %s %+v", err, texts(others), said)
	}
	got, _ := s.Get("s1", 3)
	if got.Ref != "b:s1:20.0" || len(got.D.Files) != 1 {
		t.Errorf("found in the record: %+v", got)
	}
	if fin := s.Final(said); !fin.D.Final {
		t.Error("not final")
	}
	if ms, _ := s.List("s1", 0, -1, 0); texts(ms) != "1:said:old 2:user:q 3:said:answer 4:said:next " {
		t.Errorf("all: %s", texts(ms))
	}
	// Taken back: gone from the list, its number not used again; sent again, a new message.
	s.Unsay("s1", 2)
	m, isNew, _ = s.Say(Message{ChannelMessage: store.ChannelMessage{Channel: "s1", Author: "u1", Kind: KindUser, Text: "q", Ref: "in1"}})
	if !isNew || m.Seq != 5 {
		t.Errorf("after taking back: %v %+v", isNew, m)
	}
}

// What an agent did between two things it said goes with what it says next; what it has done
// since it last spoke is kept till it speaks.
func TestActivity(t *testing.T) {
	s, r, _ := rig(t)
	r.add("s1", api.Block{Kind: api.BlockKindUser, Text: "go", ID: "in1", From: "web", FromTerminal: "t1"})
	r.add("s1", api.Block{Kind: api.BlockKindTool, Name: "Bash", Input: `{"command":"go test ./..."}`, Time: 1000})
	r.add("s1", api.Block{Kind: api.BlockKindTool, Name: "Edit", Input: `{"file_path":"/w/a.go","old_string":"a","new_string":"a\nb"}`})
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "fixed", Time: 4000})
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "and done", Time: 5000})
	r.add("s1", api.Block{Kind: api.BlockKindTool, Name: "Read", Input: `{"file_path":"/w/a.go"}`})
	ms, err := s.List("s1", 0, -1, 0)
	if err != nil || len(ms) != 3 {
		t.Fatalf("read in: %v %s", err, texts(ms))
	}
	a := ms[1].D.Activity
	if a == nil || a.Bash != 1 || a.Cmds["go"] != 1 || len(a.Files) != 1 || a.Files[0].Add != 2 || a.Files[0].Del != 1 || a.Ms != 3000 {
		t.Fatalf("what it did before speaking: %+v", a)
	}
	if ms[2].D.Activity != nil {
		t.Errorf("nothing done, yet: %+v", ms[2].D.Activity)
	}
	if d := s.Doing("s1"); d.Read != 1 || d.Bash != 0 {
		t.Errorf("doing: %+v", d)
	}
	// It speaks again: what it was doing goes with that, and nothing is left.
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "read it"})
	ms, _ = s.List("s1", 0, -1, 0)
	if a := ms[3].D.Activity; a == nil || a.Read != 1 || !s.Doing("s1").Empty() {
		t.Errorf("after speaking: %+v, doing %+v", a, s.Doing("s1"))
	}
}

// Messages are found by their words: Chinese by any two characters in a row, paths by their
// parts, and by the files their agent changed.
func TestSearch(t *testing.T) {
	s, r, _ := rig(t)
	r.add("s1", api.Block{Kind: api.BlockKindUser, Text: "看一下上下文压缩的权限问题", ID: "in1", From: "web", FromTerminal: "t1"})
	r.add("s1", api.Block{Kind: api.BlockKindTool, Name: "Edit", Input: `{"file_path":"/w/internal/store/store.go","old_string":"a","new_string":"b"}`})
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "Fixed the Passkey gate."})
	r.add("s1", api.Block{Kind: api.BlockKindAssistant, Text: "无关的一句"})
	for words, want := range map[string]string{
		"权限":         "1:user:看一下上下文压缩的权限问题 ",
		"上下文压缩":      "1:user:看一下上下文压缩的权限问题 ",
		"压缩 权限":      "1:user:看一下上下文压缩的权限问题 ",
		"文上":         "",
		"passkey":    "2:said:Fixed the Passkey gate. ",
		"pass":       "2:said:Fixed the Passkey gate. ",
		"store.go":   "2:said:Fixed the Passkey gate. ",
		"gate 权限":    "",
		"句":          "3:said:无关的一句 ",
		`"; DROP --`: "",
	} {
		ms, err := s.Search("s1", words, 0)
		if err != nil || texts(ms) != want {
			t.Errorf("%q: %v %q, want %q", words, err, texts(ms), want)
		}
	}
	// Taken back: not found.
	s.Unsay("s1", 1)
	if ms, _ := s.Search("s1", "权限", 0); len(ms) != 0 {
		t.Errorf("found what was taken back: %s", texts(ms))
	}
}

// A session is in other channels by subscription: its nick is its own and the same in each, no
// two agents' alike; what its record has is said in its own channel only. A channel may be no
// session's: then it has no agent until one joins.
func TestSubscriptions(t *testing.T) {
	s, r, st := rig(t)
	base := func(string) string { return "Claude" }
	for _, id := range []string{"a", "b", "c"} {
		r.blocks[id] = nil
	}
	if s.Speaks("a", "b") || !s.Speaks("a", "a") || len(s.In("b")) != 0 {
		t.Fatalf("before anyone joined")
	}
	m, err := s.Join("a", "b", nil, "", base, nil)
	if err != nil || m.Nick != "claude2" || m.Triggers != "[]" || m.Direct() {
		t.Fatalf("join: %+v %v", m, err)
	}
	if _, err := s.Join("a", "b", nil, "", base, nil); !errors.Is(err, ErrMember) {
		t.Errorf("joining twice: %v", err)
	}
	if _, err := s.Join("a", "a", nil, "", base, nil); !errors.Is(err, ErrMember) {
		t.Errorf("joining its own: %v", err)
	}
	ms, _ := s.Members("a")
	if len(ms) != 3 || ms[1].Nick != "claude" || !ms[1].Direct() || ms[2].Session != "b" {
		t.Fatalf("members: %+v", ms)
	}
	// The same nick in another channel; a third agent's is its own.
	if m, err := s.Join("c", "b", EveryoneOn, "", base, nil); err != nil || m.Nick != "claude2" {
		t.Fatalf("join another: %+v %v", m, err)
	}
	if s.Nick("c") != "claude3" || fmt.Sprint(s.In("b")) != "[a c]" || !s.Speaks("c", "b") {
		t.Errorf("nicks and channels: %q %v", s.Nick("c"), s.In("b"))
	}
	if err := s.SetNick("c", "CLAUDE2"); !errors.Is(err, ErrNick) {
		t.Errorf("another's nick, in other case: %v", err)
	}
	if err := s.SetNick("c", "lead"); err != nil || s.Nick("c") != "lead" {
		t.Errorf("a new nick: %v %q", err, s.Nick("c"))
	}
	// What b says in its turn is its own channel's, not a's.
	r.add("b", api.Block{Kind: api.BlockKindUser, Text: "#3: hello\n\n<channel-context>\nYou are claude2\n</channel-context>", From: "web"})
	r.add("b", api.Block{Kind: api.BlockKindAssistant, Text: "working on it"})
	added, err := s.Sync("b")
	if err != nil || texts(added) != "1:said:working on it " || added[0].Channel != "b" {
		t.Fatalf("sync: %q %v", texts(added), err)
	}
	if n := s.End("a"); n != 0 {
		t.Errorf("said in the channel it is in: %d", n)
	}
	if err := s.Leave("a", store.AgentMember("b")); err != nil || s.Speaks("a", "b") {
		t.Errorf("leave: %v", err)
	}
	if err := s.Leave("a", store.AgentMember("a")); !errors.Is(err, ErrNoMember) {
		t.Errorf("its own agent leaving: %v", err)
	}

	// A channel of its own.
	if err := s.Create(store.Channel{ID: "room", Name: "Room", Cwd: "/w"}); err != nil {
		t.Fatal(err)
	}
	if c, ok := s.Own("room"); !ok || c.Name != "Room" || s.Speaks("room", "room") {
		t.Fatalf("own: %+v %v", c, ok)
	}
	if ms, _ := s.Members("room"); len(ms) != 1 || ms[0].Member != Everyone || len(s.Agents("room")) != 0 {
		t.Errorf("a new channel's members: %+v", ms)
	}
	if _, err := s.Join("room", "a", nil, "", base, nil); err != nil || fmt.Sprint(s.Agents("room")) != "[a]" {
		t.Errorf("joining a channel of its own: %v %v", err, s.Agents("room"))
	}
	if _, _, err := s.Say(Message{ChannelMessage: store.ChannelMessage{Channel: "room", Author: "web", Kind: KindUser, Text: "hi"}}); err != nil || s.End("room") != 1 {
		t.Errorf("say: %v", err)
	}
	if err := s.Remove("room"); err != nil || len(s.In("a")) != 0 {
		t.Errorf("remove: %v %v", err, s.In("a"))
	}
	if _, ok := s.Own("room"); ok {
		t.Errorf("still there")
	}
	// Reading the store anew gives the same.
	s2 := New(st, r, t.Logf)
	s2.Numbered = true
	if s2.Nick("b") != "claude2" || fmt.Sprint(s2.In("b")) != "[c]" {
		t.Errorf("read anew: %q %v", s2.Nick("b"), s2.In("b"))
	}
}

// A nick given by default: the harness's name and four hex digits of the session, more while
// taken (kumo, 2026-11-16: not claude2); one so made is told apart from one chosen.
func TestDefaultNick(t *testing.T) {
	taken := map[string]bool{"Claude-b134": true}
	if n := DefaultNick("claude", "b134e2f0-aaaa", func(n string) bool { return taken[n] }); n != "Claude-b134e" {
		t.Errorf("taken: %q", n)
	}
	if n := DefaultNick("dsh", "session-c54bdf12", func(string) bool { return false }); n != "DeepSeek-c54b" {
		t.Errorf("dsh: %q", n)
	}
	if n := NickWithID("克劳德大小姐", "e4446f91-bd02", func(string) bool { return false }); n != "克劳德大小姐-e444" {
		t.Errorf("a role's name taken: %q", n)
	}
	if n := NickWithID(strings.Repeat("长", 32), "e4446f91", func(string) bool { return false }); len([]rune(n)) != 32 || !strings.HasSuffix(n, "长-e444") {
		t.Errorf("a long name taken: %q", n)
	}
	few := func(n string) bool { return n == "tester-ab12" }
	if n := NickWithID("tester", "xyz-ab12", few); len(n) != len("tester-0000") || !strings.HasPrefix(n, "tester-") || n == "tester-ab12" || strings.Trim(n[7:], "0123456789abcdef") != "" {
		t.Errorf("an id of few digits, all taken: %q", n)
	}
	if IsDefaultNick("Fable-fd54") {
		t.Error("a role's name with digits after it taken for one of Iridium's")
	}
	if !IsDefaultNick(DefaultNick("agy", "d04a1234", func(string) bool { return false })) || !IsDefaultNick("Neon-d04a") || !IsDefaultNick("Hydrogen-a13e") {
		t.Error("agy's default nicks")
	}
	if !IsDefaultNick("Claude-b134") || IsDefaultNick("大肥鱼") || IsDefaultNick("claude2") {
		t.Error("IsDefaultNick")
	}
}

// A row's subscriptions read from what rows kept before them, and checked as they are set.
func TestTriggersOf(t *testing.T) {
	for raw, want := range map[string]string{
		"all":                              `[{"message":{"from":["*"]}}]`,
		"at":                               `[]`,
		"":                                 `[]`,
		`["*","agent:x"]`:                  `[{"message":{"from":["*","agent:x"]}}]`,
		`[{"event":["join"],"text":"hi"}]`: `[{"event":["join"],"text":"hi"}]`,
	} {
		if got := EncodeTriggers(TriggersOf(raw)); got != want {
			t.Errorf("%q: %s, want %s", raw, got, want)
		}
	}
	on, err := CleanOn([]api.Trigger{{Event: []string{"people", "join"}}, {Message: &api.MessageTrigger{Words: []string{" a ", "a"}}}}, "agent:me")
	if err != nil || EncodeTriggers(on) != `[{"event":["join"]},{"message":{"words":["a"]}}]` {
		t.Fatalf("%s %v", EncodeTriggers(on), err)
	}
	for _, bad := range [][]api.Trigger{
		{{Event: []string{"joinn"}}},
		{{Message: &api.MessageTrigger{From: []string{"agent:me"}}}},
		{{Cron: "61 * * * *"}},
		{{Cron: "0 9 * * *", Command: "x"}},
		{{}},
	} {
		if _, err := CleanOn(bad, "agent:me"); !errors.Is(err, ErrOn) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}
