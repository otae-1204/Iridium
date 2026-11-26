// Package testagent holds the fake agents tests run instead of real ones. Only test files import
// it, so it is never part of the binary. A test binary dispatches to them from its TestMain.
package testagent

import (
	"bufio"
	"cmp"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kumoSleeping/iridium/internal/hold"
)

// titleLine is the session's or a channel's title put after a message now and then
// (assemble.TitleNow): not its words.
var titleLine = regexp.MustCompile(`\n\n(?:Session|Channel \S+) title: `)

// TinyPNG is a 2×1 black PNG (base64), the picture a PICTURE turn's Read gives; TinyHandle names
// it in an agent's own store.
const (
	TinyPNG    = "iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAIAAAB7QOjdAAAAC0lEQVR4nGNgAAMAAAcAAbKGrPQAAAAASUVORK5CYII="
	TinyHandle = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

// Main runs what the test binary was started as, if not the tests: the holder (`hold ...`), or
// the fake Claude through a link named claude. ok false: run the tests.
func Main() (code int, ok bool) {
	if filepath.Base(os.Args[0]) == "claude" {
		return Claude(), true
	}
	if filepath.Base(os.Args[0]) == "codex" {
		return Codex(), true
	}
	if filepath.Base(os.Args[0]) == "agy" {
		return Agy(), true
	}
	if os.Getenv("IRIDIUM_FAKE_DSH") != "" {
		return Dsh(), true
	}
	if len(os.Args) > 1 && os.Args[1] == "hold" {
		return hold.Main(os.Args[2:]), true
	}
	return 0, false
}

// Claude is a fake claude -p. It answers initialize, get_usage, get_context_usage, interrupt,
// get_settings, apply_flag_settings, set_model and set_permission_mode; the model, the effort level
// and ultracode are kept as --model, --effort, --settings and those set them (default runs
// fake-model, haiku fake-haiku, which takes no effort level and no ultracode). A message runs a turn
// that says "ok: <text>" (its source prefix taken off), costs $0.01 more, and goes into a
// transcript under $CLAUDE_CONFIG_DIR. By how the message ends:
//
//   - ASK asks to run `ls` first and goes on once answered; ASKASK then asks for `ls -la` too.
//     Both suggest the rule "ls *".
//   - QUESTION asks two questions (AskUserQuestion: a color, one of Red or Blue; sizes, any of S,
//     M, L) and answers "ok: answers <question>=<answer>; ...", by question.
//   - WAIT runs for half a second (an interrupt ends it at once); messages that come meanwhile
//     join it, each answered "ok: <text>" in the same turn. HOLD is WAIT until the first message
//     joins it (ten seconds at most), however slow the machine. One that comes meanwhile ending in
//     LATER waits in the agent instead, as Claude's queue does while a call runs: once the turn
//     ends (an interrupt too), it runs as the next turn, LATER taken off.
//   - crash begins the turn, then exits 3.
//   - STREAM writes a turn as a real one comes: words, two calls and a long answer, piece by piece.
//     FASTSTREAM writes the same pieces without the pauses between the letters (for tests); a
//     call still takes its time.
//   - QUOTA is refused for the quota, which comes back in an hour.
//   - BACKGROUND sends a subagent and a shell command to the background and ends the turn: the
//     subagent goes on till stop_task stops it, the command ends after bgBash.
//   - BGREPORT sends a shell command to the background and ends the turn; the command ends after
//     bgReport, and the agent begins a turn of its own (no message) that says "report: <task>",
//     as Claude does when a task of its ends. BGSTAY: the same, the command going on till
//     stop_task stops it. BGQUIET: as BGSTAY, and no turn follows its end.
//
// /compact compacts for three seconds (a compacting status, then the boundary with its figures).
// /clear (as the whole message, its source prefix aside) clears it as Claude's own does: a
// conversation_reset, then it goes on in the same process as a conversation of a new id, the
// command its first lines.
// compactFor is how long a /compact takes.
var compactFor = 3 * time.Second

// bgBash is how long BACKGROUND's shell command runs: IRIDIUM_FAKE_BG_BASH (a duration), else 8s.
var bgBash = func() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("IRIDIUM_FAKE_BG_BASH")); err == nil {
		return d
	}
	return 8 * time.Second
}()

// bgReport is how long BGREPORT's shell command runs: IRIDIUM_FAKE_BG_REPORT (a duration), else
// half a second.
var bgReport = func() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("IRIDIUM_FAKE_BG_REPORT")); err == nil {
		return d
	}
	return 500 * time.Millisecond
}()

func Claude() int {
	session := ""
	for _, a := range os.Args[1:] {
		if v, ok := strings.CutPrefix(a, "--session-id="); ok {
			session = v
		}
		if v, ok := strings.CutPrefix(a, "--resume="); ok {
			session = v
		}
	}
	model, effort, ultra := cmp.Or(argOf(os.Args, "--model"), "default"), cmp.Or(argOf(os.Args, "--effort"), "medium"), false
	if v := argOf(os.Args, "--settings"); v != "" {
		var st struct {
			Ultracode bool `json:"ultracode"`
		}
		json.Unmarshal([]byte(v), &st)
		ultra = st.Ultracode
	}
	resolved := func() string {
		if model == "haiku" {
			return "fake-haiku"
		}
		return "fake-model"
	}
	cwd, _ := os.Getwd()
	dir := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "projects", strings.ReplaceAll(cwd, "/", "-"))
	os.MkdirAll(dir, 0o700)
	recordPath := filepath.Join(dir, session+".jsonl")
	record, _ := os.OpenFile(recordPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	out := bufio.NewWriter(os.Stdout)
	say := func(v any) {
		b, _ := json.Marshal(v)
		out.Write(append(b, '\n'))
		out.Flush()
	}
	note := func(v map[string]any) {
		// Each line says where and which session, as Claude's own do.
		v["cwd"], v["sessionId"] = cwd, session
		b, _ := json.Marshal(v)
		record.Write(append(b, '\n'))
	}
	answer := func(id string, body any) {
		say(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": id, "response": body}})
	}
	cost := 0.0
	// A resumed session goes on from its running total, as Claude's does.
	if b, err := os.ReadFile(filepath.Join(dir, session+".cost")); err == nil {
		fmt.Sscan(string(b), &cost)
	}
	usage := func() map[string]any {
		return map[string]any{"fake-model": map[string]any{"inputTokens": int(cost * 1000), "outputTokens": int(cost * 100), "cacheReadInputTokens": 0, "cacheCreationInputTokens": 0, "costUSD": cost}}
	}
	textOf := func(f map[string]any) string {
		return f["message"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	}
	unprefixed := func(text string) string {
		// The session's title put after a message now and then (assemble.TitleNow): not its words.
		if i := titleLine.FindStringIndex(text); i != nil {
			text = text[:i[0]]
		}
		if _, rest, ok := strings.Cut(text, "] "); ok && strings.HasPrefix(text, "[") {
			return rest
		}
		return text
	}
	stamp := func() string { return time.Now().UTC().Format(time.RFC3339Nano) }
	assistant := func(reply string) {
		for _, d := range []string{reply[:min(4, len(reply))], reply[min(4, len(reply)):]} {
			say(map[string]any{"type": "stream_event", "parent_tool_use_id": nil, "event": map[string]any{"type": "content_block_delta", "delta": map[string]any{"type": "text_delta", "text": d}}})
		}
		say(map[string]any{"type": "assistant", "parent_tool_use_id": nil, "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": reply}}, "usage": map[string]any{"input_tokens": 10}}})
		note(map[string]any{"type": "assistant", "timestamp": stamp(), "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": reply}}}})
	}
	begin := func(f map[string]any) {
		say(map[string]any{"type": "system", "subtype": "session_state_changed", "state": "running"})
		if f != nil {
			say(map[string]any{"type": "command_lifecycle", "command_uuid": f["uuid"], "state": "started"})
		}
		say(map[string]any{"type": "system", "subtype": "init", "session_id": session, "model": resolved()})
		if f != nil {
			note(map[string]any{"type": "user", "uuid": f["uuid"], "timestamp": stamp(), "message": map[string]any{"role": "user", "content": textOf(f)}})
		}
	}
	end := func(f map[string]any, subtype string) {
		cost += 0.01
		os.WriteFile(filepath.Join(dir, session+".cost"), []byte(fmt.Sprint(cost)), 0o600)
		say(map[string]any{"type": "result", "subtype": subtype, "num_turns": 1, "usage": map[string]any{"input_tokens": 10, "output_tokens": 1}, "total_cost_usd": cost, "modelUsage": usage()})
		if f != nil {
			say(map[string]any{"type": "command_lifecycle", "command_uuid": f["uuid"], "state": "completed"})
		}
		say(map[string]any{"type": "system", "subtype": "session_state_changed", "state": "idle"})
	}
	finish := func(f map[string]any, steered []map[string]any) {
		// "(say nothing)" in the first line of what it was given: a turn that ends with no words.
		if first, _, _ := strings.Cut(textOf(f), "\n"); !strings.Contains(first, "(say nothing)") {
			assistant("ok: " + unprefixed(textOf(f)))
		}
		for _, s := range steered {
			assistant("ok: " + unprefixed(textOf(s)))
			say(map[string]any{"type": "command_lifecycle", "command_uuid": s["uuid"], "state": "completed"})
		}
		end(f, "success")
	}
	// open: the asks waiting for an answer, as their requests went; initialize names them again
	// (pending_permission_requests), as the CLI does for a daemon that comes back.
	open := []any{}
	ask := func(id, command string) {
		req := map[string]any{"type": "control_request", "request_id": id, "request": map[string]any{"subtype": "can_use_tool", "tool_name": "Bash", "input": map[string]any{"command": command}, "tool_use_id": "toolu_" + id,
			"permission_suggestions": []any{map[string]any{"type": "addRules", "rules": []any{map[string]any{"toolName": "Bash", "ruleContent": "ls *"}}, "behavior": "allow", "destination": "localSettings"}}}}
		open = []any{req}
		// As the CLI does: the call in an assistant message (its usage the context so far), then the ask.
		say(map[string]any{"type": "assistant", "parent_tool_use_id": nil, "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": "toolu_" + id, "name": "Bash", "input": map[string]any{"command": command}}}, "usage": map[string]any{"input_tokens": 10}}})
		say(req)
	}
	question := func(id string) {
		qs := []any{
			map[string]any{"question": "Which color?", "header": "Color", "multiSelect": false, "options": []any{map[string]any{"label": "Red"}, map[string]any{"label": "Blue", "description": "the sea"}}},
			map[string]any{"question": "Which sizes?", "header": "Sizes", "multiSelect": true, "options": []any{map[string]any{"label": "S"}, map[string]any{"label": "M"}, map[string]any{"label": "L"}}},
		}
		req := map[string]any{"type": "control_request", "request_id": id, "request": map[string]any{"subtype": "can_use_tool", "tool_name": "AskUserQuestion", "input": map[string]any{"questions": qs}, "tool_use_id": "toolu_" + id, "requires_user_interaction": true}}
		open = []any{req}
		say(req)
	}

	lines := make(chan []byte)
	go func() {
		in := bufio.NewReader(os.Stdin)
		for {
			line, err := in.ReadBytes('\n')
			if err != nil {
				close(lines)
				return
			}
			lines <- line
		}
	}()
	var pending map[string]any // the message waiting on an ask
	asked := 0
	var waiting map[string]any        // a WAIT turn
	holding := false                  // a HOLD turn: it ends once a message joins it
	steerFor := 50 * time.Millisecond // how long a held turn goes on once a message joined it
	var steered []map[string]any
	var later []map[string]any // LATER messages waiting for the turn's end
	var timer <-chan time.Time
	var compacting map[string]any // a /compact running
	var compacted <-chan time.Time
	// BACKGROUND's tasks: a subagent that goes on till stopped (its progress each second), and a
	// shell command that ends after bgBash.
	var bg []map[string]any
	var bgTick <-chan time.Time
	bgRuns := 0
	bgSince := map[string]time.Time{}  // a subagent's id: when it began
	bgShells := map[string]time.Time{} // a shell command's id: when it ends
	bgReports := map[string]bool{}     // a task's id: its end begins a turn that reports it
	var bgDue <-chan time.Time         // BGREPORT's command ends
	bgDueID := ""
	tellBg := func() {
		say(map[string]any{"type": "system", "subtype": "background_tasks_changed", "tasks": bg, "uuid": "bg", "session_id": session})
	}
	dropBg := func(id, status string) bool {
		for i, x := range bg {
			if x["task_id"] == id {
				bg = append(bg[:i], bg[i+1:]...)
				say(map[string]any{"type": "system", "subtype": "task_notification", "task_id": id, "status": status, "summary": "done"})
				tellBg()
				return true
			}
		}
		return false
	}
	// report: the turn the agent begins by itself as task id ends, saying so.
	report := func(id string) {
		if !bgReports[id] {
			return
		}
		delete(bgReports, id)
		begin(nil)
		assistant("report: " + id)
		end(nil, "success")
	}
	for {
		var line []byte
		if len(later) > 0 && waiting == nil && pending == nil && compacting == nil {
			f := later[0]
			later = later[1:]
			c := f["message"].(map[string]any)["content"].([]any)[0].(map[string]any)
			c["text"] = strings.Replace(c["text"].(string), " LATER", "", 1)
			line, _ = json.Marshal(f)
			goto parse
		}
		select {
		case l, ok := <-lines:
			if !ok {
				return 0
			}
			line = l
		case <-timer:
			timer = nil
			finish(waiting, steered)
			waiting, steered, holding = nil, nil, false
			continue
		case <-bgDue:
			bgDue = nil
			if dropBg(bgDueID, "completed") {
				report(bgDueID)
			}
			continue
		case now := <-bgTick:
			for _, x := range bg {
				if x["task_type"] == "local_agent" {
					ms := now.Sub(bgSince[x["task_id"].(string)]).Milliseconds()
					n := int(ms/1000) + 1
					say(map[string]any{"type": "system", "subtype": "task_progress", "task_id": x["task_id"], "description": fmt.Sprintf("Reading file %d", n), "usage": map[string]any{"total_tokens": 100 * n, "tool_uses": n, "duration_ms": ms}})
				}
			}
			for id, at := range bgShells {
				if now.After(at) {
					delete(bgShells, id)
					dropBg(id, "completed")
				}
			}
			continue
		case <-compacted:
			compacted = nil
			pre, post := 120000, 9000
			note(map[string]any{"type": "system", "subtype": "compact_boundary", "timestamp": stamp(), "content": "Conversation compacted", "compactMetadata": map[string]any{"trigger": "manual", "preTokens": pre, "postTokens": post, "durationMs": compactFor.Milliseconds()}})
			note(map[string]any{"type": "user", "isCompactSummary": true, "timestamp": stamp(), "message": map[string]any{"role": "user", "content": "This session is being continued from a previous conversation."}})
			note(map[string]any{"type": "user", "uuid": compacting["uuid"], "timestamp": stamp(), "message": map[string]any{"role": "user", "content": "<command-name>/compact</command-name>\n<command-message>compact</command-message>\n<command-args></command-args>"}})
			say(map[string]any{"type": "system", "subtype": "compact_boundary", "compact_metadata": map[string]any{"trigger": "manual", "pre_tokens": pre, "post_tokens": post}})
			say(map[string]any{"type": "system", "subtype": "status", "status": nil, "compact_result": "success"})
			end(compacting, "success")
			compacting = nil
			continue
		}
	parse:
		var f map[string]any
		if json.Unmarshal(line, &f) != nil {
			return 1 // as the real CLI does
		}
		switch f["type"] {
		case "control_request":
			id := f["request_id"].(string)
			req := f["request"].(map[string]any)
			switch req["subtype"] {
			case "initialize":
				say(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": id, "pending_permission_requests": open,
					"response": map[string]any{"claude_code_version": "fake", "current_permission_mode": modeOf(os.Args), "models": []any{
						map[string]any{"value": "default", "resolvedModel": "fake-model", "displayName": "Default (recommended)", "description": "Fake 1 · For tests", "supportsEffort": true, "supportedEffortLevels": []string{"low", "medium", "high", "xhigh", "max"}},
						map[string]any{"value": "haiku", "resolvedModel": "fake-haiku", "displayName": "Haiku"}}, "commands": []any{map[string]any{"name": "compact", "description": "Compact the conversation", "argumentHint": "<instructions>"}},
						"account": map[string]any{"email": "user@example.com", "subscriptionType": "Claude Pro"}}}})
			case "get_usage":
				answer(id, map[string]any{"session": map[string]any{"total_cost_usd": cost, "model_usage": usage()}, "rate_limits": map[string]any{"limits": []any{map[string]any{"group": "session", "percent": 10, "resets_at": "2026-01-01T00:00:00Z"}}}})
			case "get_context_usage":
				answer(id, map[string]any{"totalTokens": 100, "maxTokens": 1000, "categories": []any{map[string]any{"id": "messages", "name": "Messages", "tokens": 100, "kind": "used"}}})
			case "get_settings":
				avail := model != "haiku"
				answer(id, map[string]any{"applied": map[string]any{"model": resolved(), "effort": effort, "ultracode": ultra && avail, "ultracodeRequested": ultra, "ultracodeAvailable": avail}})
			case "apply_flag_settings":
				st, _ := req["settings"].(map[string]any)
				if v, ok := st["effortLevel"].(string); ok {
					effort = v
				}
				if v, ok := st["ultracode"].(bool); ok {
					ultra = v
				}
				answer(id, map[string]any{})
			case "set_model":
				model, _ = req["model"].(string)
				answer(id, map[string]any{})
			case "set_permission_mode":
				answer(id, map[string]any{})
			case "stop_task":
				tid, _ := req["task_id"].(string)
				if !dropBg(tid, "killed") {
					say(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": id, "error": "no task " + tid}})
					break
				}
				delete(bgShells, tid)
				answer(id, map[string]any{})
				report(tid)
			case "interrupt":
				answer(id, map[string]any{"still_queued": []any{}})
				if waiting != nil {
					timer = nil
					note(map[string]any{"type": "user", "timestamp": stamp(), "message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "[Request interrupted by user]"}}}})
					end(waiting, "error_during_execution")
					for _, s := range steered {
						say(map[string]any{"type": "command_lifecycle", "command_uuid": s["uuid"], "state": "completed"})
					}
					waiting, steered, holding = nil, nil, false
				}
			}
		case "control_response":
			if pending != nil {
				open = []any{}
				text := textOf(pending)
				if strings.HasSuffix(text, "QUESTION") {
					var r struct {
						Response struct {
							Response struct {
								UpdatedInput struct {
									Answers map[string]string `json:"answers"`
								} `json:"updatedInput"`
							} `json:"response"`
						} `json:"response"`
					}
					json.Unmarshal(line, &r)
					var said []string
					for q, a := range r.Response.Response.UpdatedInput.Answers {
						said = append(said, q+"="+a)
					}
					sort.Strings(said)
					assistant("ok: answers " + strings.Join(said, "; "))
					end(pending, "success")
					pending = nil
					continue
				}
				if strings.HasSuffix(text, "ASKASK") && asked == 1 {
					asked++
					ask("ask-2", "ls -la")
					continue
				}
				finish(pending, nil)
				pending = nil
			}
		case "user":
			text := textOf(f)
			if words, _, _ := strings.Cut(text, "\n\n<channel-context>"); waiting != nil && strings.HasSuffix(words, " LATER") {
				say(map[string]any{"type": "command_lifecycle", "command_uuid": f["uuid"], "state": "queued"})
				later = append(later, f)
				continue
			}
			if waiting != nil {
				// Taken into the running turn, as the CLI does with a message mid-turn.
				say(map[string]any{"type": "command_lifecycle", "command_uuid": f["uuid"], "state": "started"})
				note(map[string]any{"type": "user", "uuid": f["uuid"], "timestamp": stamp(), "message": map[string]any{"role": "user", "content": text}})
				steered = append(steered, f)
				if holding {
					holding, timer = false, time.After(steerFor)
				}
				continue
			}
			if unprefixed(text) == "/compact" {
				// Compacts for compactFor, as Claude's does: a status, then the boundary and the
				// command's line in the record.
				say(map[string]any{"type": "system", "subtype": "session_state_changed", "state": "running"})
				say(map[string]any{"type": "command_lifecycle", "command_uuid": f["uuid"], "state": "started"})
				say(map[string]any{"type": "system", "subtype": "status", "status": "compacting"})
				compacting, compacted = f, time.After(compactFor)
				continue
			}
			if unprefixed(text) == "/clear" {
				say(map[string]any{"type": "system", "subtype": "session_state_changed", "state": "running"})
				say(map[string]any{"type": "command_lifecycle", "command_uuid": f["uuid"], "state": "started"})
				say(map[string]any{"type": "conversation_reset", "new_conversation_id": f["uuid"], "trigger": "clear", "session_id": session})
				b := make([]byte, 16)
				rand.Read(b)
				b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
				session = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
				record.Close()
				recordPath = filepath.Join(dir, session+".jsonl")
				record, _ = os.OpenFile(recordPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
				say(map[string]any{"type": "system", "subtype": "init", "session_id": session, "model": resolved()})
				note(map[string]any{"type": "user", "uuid": f["uuid"], "timestamp": stamp(), "message": map[string]any{"role": "user", "content": "<command-name>/clear</command-name>\n<command-message>clear</command-message>\n<command-args></command-args>"}})
				end(f, "success")
				continue
			}
			begin(f)
			// A session's title put after a message now and then (assemble.TitleNow): not its words.
			if i := titleLine.FindStringIndex(text); i != nil {
				text = text[:i[0]]
			}
			// A channel's message: its words are before its context.
			if i := strings.Index(text, "\n\n<channel-context>"); i >= 0 {
				text = text[:i]
			}
			switch {
			case strings.HasSuffix(text, "crash"):
				record.Close()
				return 3
			case strings.HasSuffix(text, "QUESTION"):
				pending, asked = f, 1
				question("ask-q")
				continue
			case strings.HasSuffix(text, "ASK"):
				pending, asked = f, 1
				ask("ask-1", "ls")
				continue
			case strings.HasSuffix(text, "WAIT"):
				waiting = f
				timer = time.After(500 * time.Millisecond)
				continue
			case strings.HasSuffix(text, "WORKHOLD"):
				// Two calls run, then held as HOLD is: a message sent meanwhile is taken into a turn
				// that has done something already.
				for i, cmd := range []string{"ls", "pwd"} {
					id := fmt.Sprintf("toolu_wh%d_%d", i, time.Now().UnixNano())
					call := map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": cmd, "description": "look"}}
					done := map[string]any{"type": "tool_result", "tool_use_id": id, "content": "a.txt"}
					for _, m := range []map[string]any{{"role": "assistant", "content": []any{call}}, {"role": "user", "content": []any{done}}} {
						say(map[string]any{"type": m["role"], "parent_tool_use_id": nil, "message": m})
						note(map[string]any{"type": m["role"], "timestamp": stamp(), "message": m})
					}
				}
				waiting, holding, steerFor = f, true, 3*time.Second
				timer = time.After(20 * time.Second)
				continue
			case strings.HasSuffix(text, "HOLD"):
				waiting, holding, steerFor = f, true, 50*time.Millisecond
				timer = time.After(10 * time.Second)
				continue
			case strings.HasSuffix(text, "DENIED"):
				// Auto mode's classifier turns the call down: the CLI answers the model itself.
				call := map[string]any{"type": "tool_use", "id": "toolu_denied", "name": "Bash", "input": map[string]any{"command": "npx -y some-tool", "description": "fetch a tool"}}
				say(map[string]any{"type": "assistant", "parent_tool_use_id": nil, "message": map[string]any{"role": "assistant", "content": []any{call}, "usage": map[string]any{"input_tokens": 10}}})
				note(map[string]any{"type": "assistant", "timestamp": stamp(), "message": map[string]any{"role": "assistant", "content": []any{call}}})
				result := map[string]any{"type": "tool_result", "tool_use_id": "toolu_denied", "is_error": true, "content": "Permission for this action was denied by the Claude Code auto mode classifier. Reason: [Code from External]. If you have other tasks that don't depend on this action, continue working on those."}
				say(map[string]any{"type": "user", "parent_tool_use_id": nil, "message": map[string]any{"role": "user", "content": []any{result}}})
				note(map[string]any{"type": "user", "timestamp": stamp(), "message": map[string]any{"role": "user", "content": []any{result}}})
				finish(f, nil)
				continue
			case strings.HasSuffix(text, "AROUND"):
				// Denied, then Claude goes another way: a call that runs.
				call := map[string]any{"type": "tool_use", "id": "toolu_denied", "name": "Bash", "input": map[string]any{"command": "npx -y some-tool", "description": "fetch a tool"}}
				result := map[string]any{"type": "tool_result", "tool_use_id": "toolu_denied", "is_error": true, "content": "Permission for this action was denied by the Claude Code auto mode classifier. Reason: [Code from External]."}
				other := map[string]any{"type": "tool_use", "id": "toolu_other", "name": "Bash", "input": map[string]any{"command": "ls", "description": "list"}}
				done := map[string]any{"type": "tool_result", "tool_use_id": "toolu_other", "content": "a.txt"}
				for _, m := range []map[string]any{{"role": "assistant", "content": []any{call}}, {"role": "user", "content": []any{result}}, {"role": "assistant", "content": []any{other}}, {"role": "user", "content": []any{done}}} {
					say(map[string]any{"type": m["role"], "parent_tool_use_id": nil, "message": m})
					note(map[string]any{"type": m["role"], "timestamp": stamp(), "message": m})
				}
				finish(f, nil)
				continue
			case strings.HasSuffix(text, "STREAM"):
				// A turn as a real one comes: a line of words, two calls written piece by piece
				// and run, then a long answer in Markdown, each a few letters at a time.
				delta := func(d map[string]any) {
					say(map[string]any{"type": "stream_event", "parent_tool_use_id": nil, "event": map[string]any{"type": "content_block_delta", "delta": d}})
					if !strings.HasSuffix(text, "FASTSTREAM") {
						time.Sleep(20 * time.Millisecond)
					}
				}
				words := func(reply string) {
					for r := []rune(reply); len(r) > 0; r = r[min(6, len(r)):] {
						delta(map[string]any{"type": "text_delta", "text": string(r[:min(6, len(r))])})
					}
					m := map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": reply}}}
					say(map[string]any{"type": "assistant", "parent_tool_use_id": nil, "message": m})
					note(map[string]any{"type": "assistant", "timestamp": stamp(), "message": m})
				}
				call := func(id, name string, input map[string]any, result string) {
					say(map[string]any{"type": "stream_event", "parent_tool_use_id": nil, "event": map[string]any{"type": "content_block_start", "content_block": map[string]any{"type": "tool_use", "id": id, "name": name}}})
					b, _ := json.Marshal(input)
					for s := string(b); len(s) > 0; s = s[min(8, len(s)):] {
						delta(map[string]any{"type": "input_json_delta", "partial_json": s[:min(8, len(s))]})
					}
					use := map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}}}
					say(map[string]any{"type": "assistant", "parent_tool_use_id": nil, "message": use})
					note(map[string]any{"type": "assistant", "timestamp": stamp(), "message": use})
					time.Sleep(400 * time.Millisecond)
					res := map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": result}}}
					say(map[string]any{"type": "user", "parent_tool_use_id": nil, "message": res})
					note(map[string]any{"type": "user", "timestamp": stamp(), "message": res})
				}
				n := fmt.Sprint(time.Now().UnixNano())
				words("Looking at how the list is drawn first.")
				call("toolu_s1_"+n, "Bash", map[string]any{"command": "grep -rn \"useFlip\" web/src | head -20", "description": "find the list's motion"}, "web/src/motion.ts:36:export function useFlip")
				call("toolu_s2_"+n, "Read", map[string]any{"file_path": "/w/web/src/motion.ts"}, "export function useFlip() {}")
				var long strings.Builder
				long.WriteString("Here is what I found.\n\n")
				for i := 1; i <= 6; i++ {
					fmt.Fprintf(&long, "**Part %d.** The rows are placed by the layout, and what moves them is one animation on the column: it reads where each row was, where it is now, and plays the difference. Nothing else moves a row while that runs.\n\n- the first point of part %d, a line long enough to wrap on a narrow pane and not on a wide one\n- the second point, shorter\n\n", i, i)
				}
				long.WriteString("```ts\nexport function glide(by: number) {\n  const from = at + by;\n  return col.animate([{ transform: `translateY(${from}px)` }, { transform: \"none\" }]);\n}\n```\n\nThat is all of it.")
				words(long.String())
				end(f, "success")
				continue
			case strings.HasSuffix(text, "BACKGROUND"):
				// A subagent and a shell command sent to the background, the turn over at once: the
				// subagent (its record under subagents/, as Claude's) goes on till stopped, the
				// command ends after bgBash.
				bgRuns++
				aid, sid := fmt.Sprintf("a%sbg%d%d", session[:6], bgRuns, time.Now().UnixMilli()%100000), fmt.Sprintf("bbg%d", bgRuns)
				acall, scall := "toolu_bg_agent_"+session[:8]+aid, "toolu_bg_shell_"+session[:8]+sid
				agentCall := map[string]any{"type": "tool_use", "id": acall, "name": "Agent", "input": map[string]any{"description": "Scan the dependencies", "prompt": "List what is out of date.", "subagent_type": "general-purpose", "run_in_background": true}}
				agentDone := map[string]any{"type": "tool_result", "tool_use_id": acall, "content": "Async agent launched successfully.\nagentId: " + aid + " (internal ID - do not mention to user.)"}
				shellCall := map[string]any{"type": "tool_use", "id": scall, "name": "Bash", "input": map[string]any{"command": "npm run build", "description": "Build the web", "run_in_background": true}}
				shellDone := map[string]any{"type": "tool_result", "tool_use_id": scall, "content": "Command running in background with ID: " + sid + "."}
				subs := filepath.Join(dir, session, "subagents")
				os.MkdirAll(subs, 0o700)
				os.WriteFile(filepath.Join(subs, "agent-"+aid+".meta.json"), []byte(`{"description":"Scan the dependencies","toolUseId":"`+acall+`"}`), 0o600)
				sub, _ := json.Marshal(map[string]any{"parentUuid": nil, "isSidechain": true, "type": "user", "message": map[string]any{"role": "user", "content": "List what is out of date."}, "cwd": cwd, "sessionId": session, "agentId": aid, "timestamp": stamp()})
				os.WriteFile(filepath.Join(subs, "agent-"+aid+".jsonl"), append(sub, '\n'), 0o600)
				for _, m := range []map[string]any{{"role": "assistant", "content": []any{agentCall}}, {"role": "user", "content": []any{agentDone}}} {
					say(map[string]any{"type": m["role"], "parent_tool_use_id": nil, "message": m})
					note(map[string]any{"type": m["role"], "timestamp": stamp(), "message": m})
				}
				bg = append(bg, map[string]any{"task_id": aid, "task_type": "local_agent", "subagent_type": "general-purpose", "description": "Scan the dependencies"})
				tellBg()
				say(map[string]any{"type": "system", "subtype": "task_started", "task_id": aid, "tool_use_id": acall, "description": "Scan the dependencies", "is_backgrounded": true, "task_type": "local_agent"})
				for _, m := range []map[string]any{{"role": "assistant", "content": []any{shellCall}}, {"role": "user", "content": []any{shellDone}}} {
					say(map[string]any{"type": m["role"], "parent_tool_use_id": nil, "message": m})
					note(map[string]any{"type": m["role"], "timestamp": stamp(), "message": m})
				}
				bg = append(bg, map[string]any{"task_id": sid, "task_type": "local_bash", "description": "Build the web"})
				tellBg()
				say(map[string]any{"type": "system", "subtype": "task_started", "task_id": sid, "tool_use_id": scall, "description": "Build the web", "is_backgrounded": true, "task_type": "local_bash"})
				bgShells[sid] = time.Now().Add(bgBash)
				bgSince[aid] = time.Now()
				if bgTick == nil {
					bgTick = time.NewTicker(time.Second).C
				}
				finish(f, nil)
				continue
			case strings.HasSuffix(text, "BGREPORT"), strings.HasSuffix(text, "BGSTAY"), strings.HasSuffix(text, "BGQUIET"):
				// A shell command sent to the background, the turn over at once with its words; the
				// command's end begins a turn of the agent's own that reports it (not BGQUIET's).
				bgRuns++
				sid := fmt.Sprintf("brep%d", bgRuns)
				scall := "toolu_bg_report_" + session[:8] + sid
				shellCall := map[string]any{"type": "tool_use", "id": scall, "name": "Bash", "input": map[string]any{"command": "make", "description": "Build", "run_in_background": true}}
				shellDone := map[string]any{"type": "tool_result", "tool_use_id": scall, "content": "Command running in background with ID: " + sid + "."}
				for _, m := range []map[string]any{{"role": "assistant", "content": []any{shellCall}}, {"role": "user", "content": []any{shellDone}}} {
					say(map[string]any{"type": m["role"], "parent_tool_use_id": nil, "message": m})
					note(map[string]any{"type": m["role"], "timestamp": stamp(), "message": m})
				}
				bg = append(bg, map[string]any{"task_id": sid, "task_type": "local_bash", "description": "Build"})
				tellBg()
				say(map[string]any{"type": "system", "subtype": "task_started", "task_id": sid, "tool_use_id": scall, "description": "Build", "is_backgrounded": true, "task_type": "local_bash"})
				bgReports[sid] = !strings.HasSuffix(text, "BGQUIET")
				if strings.HasSuffix(text, "BGREPORT") {
					bgDue, bgDueID = time.After(bgReport), sid
				}
				finish(f, nil)
				continue
			case strings.HasSuffix(text, "PICTURE"):
				// A Read of a picture: its result carries the picture (TinyPNG), then one more by a
				// handle of an agent's own store, as dsh's are.
				call := map[string]any{"type": "tool_use", "id": "toolu_pic", "name": "Read", "input": map[string]any{"file_path": "/w/tiny.png"}}
				result := map[string]any{"type": "tool_result", "tool_use_id": "toolu_pic", "content": []any{
					map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": "image/png", "data": TinyPNG}},
					map[string]any{"type": "image", "attachment": map[string]any{"attachmentId": TinyHandle, "mediaType": "image/png", "width": 2, "height": 1}},
				}}
				for _, m := range []map[string]any{{"role": "assistant", "content": []any{call}}, {"role": "user", "content": []any{result}}} {
					say(map[string]any{"type": m["role"], "parent_tool_use_id": nil, "message": m})
					note(map[string]any{"type": m["role"], "timestamp": stamp(), "message": m})
				}
				finish(f, nil)
				continue
			case strings.HasSuffix(text, "AGAIN"):
				// Switched to manual for the denied call: the same call again, asked now.
				pending, asked = f, 1
				ask("again", "npx -y some-tool")
				continue
			case strings.HasSuffix(text, "QUOTA"):
				back := time.Now().Add(time.Hour).Unix()
				say(map[string]any{"type": "rate_limit_event", "rate_limit_info": map[string]any{"status": "rejected", "resetsAt": back, "rateLimitType": "five_hour"}})
				assistant("You've hit your limit")
				end(f, "error_during_execution")
				continue
			}
			finish(f, nil)
		}
	}
}

// argOf is the value after flag name in args, "" if none.
func argOf(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func modeOf(args []string) string {
	for i, a := range args {
		if a == "--permission-mode" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return "default"
}
