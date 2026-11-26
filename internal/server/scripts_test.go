package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/deliver"
)

// A channel's scripts (docs/channels.md "脚本"): kept by its members, people and agents alike;
// run by what they wait for; what they print said under their name, waking whom it names.
func TestScripts(t *testing.T) {
	t.Parallel()
	l := start(t)
	ch := l.create(200, "web", api.NewSession{Agent: api.AgentClaude, Cwd: l.work, Text: "first"})
	l.waitReply("web", ch, "ok: first")
	path := "/v1/internal/channels/" + ch + "/scripts"

	// A word that calls the agent.
	var x api.Script
	fish := api.NewScript{Name: "fish", Run: `printf '@claude 有人喊你：%s' "$IR_TEXT"`, On: []api.Trigger{{Message: &api.MessageTrigger{From: []string{"*"}, Words: []string{"大肥鱼"}}}}}
	l.want(200, "web", "POST", path, fish, &x)
	if x.Name != "fish" || !x.Enabled || x.Made.Member == "" || x.Made.At == 0 || len(x.On) != 1 {
		t.Fatalf("added: %+v", x)
	}
	l.want(409, "web", "POST", path, fish, nil)
	for _, bad := range []api.NewScript{
		{Name: "a b", Run: "true", On: []api.Trigger{{Command: "x"}}},
		{Name: "norun", On: []api.Trigger{{Command: "x"}}},
	} {
		l.want(400, "web", "POST", path, bad, nil)
	}
	for _, bad := range []api.NewScript{
		{Name: "none", Run: "true"},
		{Name: "two", Run: "true", On: []api.Trigger{{Command: "x", Cron: "* * * * *"}}},
		{Name: "cron", Run: "true", On: []api.Trigger{{Cron: "61 * * * *"}}},
		{Name: "ev", Run: "true", On: []api.Trigger{{Event: []string{"nothing"}}}},
	} {
		l.want(422, "web", "POST", path, bad, nil)
	}
	l.waitSaid("web", ch, "iridium", "添加了命令「fish」")

	_, sr := l.send("web", ch, api.Send{Text: "大肥鱼 在吗"})
	l.waitSaid("web", ch, "iridium", "有人喊你：大肥鱼 在吗")
	// One thing wakes an agent once: the words woke it, the script naming it as well does not.
	l.waitSaid("web", ch, "ok: 大肥鱼 在吗")
	time.Sleep(300 * time.Millisecond)
	for _, m := range l.said("web", ch) {
		if strings.Contains(m, "ok: ") && strings.Contains(m, "有人喊你") {
			t.Errorf("woken twice: %s", m)
		}
	}
	var runs []api.ScriptRun
	l.want(200, "web", "GET", path+"/fish/runs", nil, &runs)
	if len(runs) != 1 || runs[0].Trigger != "message" || runs[0].Seq != sr.Seq || runs[0].Code != 0 || runs[0].Said == 0 {
		t.Fatalf("runs: %+v", runs)
	}

	// A command: said in the channel, given to the script, not to the agent.
	l.want(200, "agent-in:"+ch, "POST", path, api.NewScript{Name: "echo", Run: `echo "args=$IR_ARGS by $IR_FROM"`, On: []api.Trigger{{Command: "/echo"}}}, &x)
	if x.Made.Session != ch || x.Made.Nick == "" || x.Made.Model == "" {
		t.Errorf("made by an agent: %+v", x.Made)
	}
	l.idle(ch)
	l.send("web", ch, api.Send{Text: "/echo hi there"})
	l.waitSaid("web", ch, "iridium", "args=hi there by ")
	time.Sleep(300 * time.Millisecond)
	for _, m := range l.said("web", ch) {
		if strings.Contains(m, "ok: /echo") {
			t.Errorf("the command went to the agent: %s", m)
		}
	}

	// By hand; one that fails says nothing.
	var run api.ScriptRun
	l.want(200, "web", "POST", path+"/echo/run", api.ScriptTry{Text: "x"}, &run)
	if run.Code != 0 || run.Trigger != "hand" || !strings.Contains(run.Out, "args=") {
		t.Errorf("by hand: %+v", run)
	}
	l.want(200, "web", "POST", path, api.NewScript{Name: "bad", Run: "echo oops >&2; exit 3", On: []api.Trigger{{Command: "bad"}}}, nil)
	var failed api.ScriptRun
	l.want(200, "web", "POST", path+"/bad/run", nil, &failed)
	if failed.Code != 3 || failed.Said != 0 || !strings.Contains(failed.Err, "oops") {
		t.Errorf("failed: %+v", failed)
	}

	// Its ir speaks as the script.
	var raw string
	if code := l.sat("script-in:"+ch+"/echo", "message.create", map[string]any{"channel_id": ch, "content": "said with ir"}, &raw); code != 200 {
		t.Fatalf("a script's ir: %d %s", code, raw)
	}
	l.waitSaid("web", ch, "iridium", "said with ir")

	// Off: it keeps its place and does not run.
	off := false
	l.want(200, "web", "PATCH", path+"/fish", api.ScriptPatch{Enabled: &off}, &x)
	if x.Enabled || x.Changed.Member == x.Made.Member && x.Changed.At == x.Made.At {
		t.Errorf("off: %+v", x)
	}
	l.waitSaid("web", ch, "iridium", "停用了命令「fish」")
	l.idle(ch)
	_, sr = l.send("web", ch, api.Send{Text: "大肥鱼 again"})
	l.waitSaid("web", ch, "@claude", fmt.Sprintf("ok: 大肥鱼 again"))
	time.Sleep(300 * time.Millisecond)
	for _, m := range l.said("web", ch) {
		if strings.Contains(m, "有人喊你：大肥鱼 again") {
			t.Errorf("an off script ran: %s", m)
		}
	}

	// A line of the channel's own: someone joined.
	l.want(200, "web", "POST", path, api.NewScript{Name: "hello", Run: `echo "welcome"`, On: []api.Trigger{{Event: []string{"join"}}}}, nil)
	l.want(200, "web", "POST", "/v1/internal/channels/"+ch+"/members", api.NewMember{Agent: api.AgentClaude, Nick: "guest"}, nil)
	l.waitSaid("web", ch, "iridium", "welcome")

	// A time.
	l.want(200, "web", "POST", path, api.NewScript{Name: "tick", Run: `echo "tick $IR_TRIGGER"`, On: []api.Trigger{{Cron: "*/5 * * * *"}}}, nil)
	l.s.cronTick(time.Date(2026, 11, 17, 9, 3, 0, 0, time.Local))
	l.s.cronTick(time.Date(2026, 11, 17, 9, 5, 0, 0, time.Local))
	l.s.cronTick(time.Date(2026, 11, 17, 9, 5, 0, 0, time.Local))
	l.waitSaid("web", ch, "iridium", "tick cron")
	time.Sleep(300 * time.Millisecond)
	n := 0
	for _, m := range l.said("web", ch) {
		if strings.Contains(m, "tick cron") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the time fired %d times", n)
	}

	var all []api.Script
	l.want(200, "web", "GET", path, nil, &all)
	if len(all) != 5 || all[0].Name != "fish" || all[0].Last == nil {
		t.Errorf("list: %+v", all)
	}
	l.want(200, "web", "DELETE", path+"/bad", nil, nil)
	l.want(422, "web", "GET", path+"/bad/runs", nil, nil)
	l.waitSaid("web", ch, "iridium", "删除了命令「bad」")
}

// A script and an agent waking each other stop as two agents do: the agent's turns of words
// alone counted, MaxIdle of them in a row (a script's own runs not).
func TestScriptHops(t *testing.T) {
	t.Parallel()
	l := start(t)
	ch := l.create(200, "web", api.NewSession{Agent: api.AgentClaude, Cwd: l.work, Text: "first"})
	l.waitReply("web", ch, "ok: first")
	l.want(200, "web", "POST", "/v1/internal/channels/"+ch+"/scripts", api.NewScript{Name: "again", Run: `echo "@claude again"`,
		On: []api.Trigger{{Message: &api.MessageTrigger{From: []string{"agent:" + ch}}}}}, nil)
	l.send("web", ch, api.Send{Text: "go"})
	l.waitSaid("web", ch, "已暂停", fmt.Sprint(deliver.MaxIdle))
}
