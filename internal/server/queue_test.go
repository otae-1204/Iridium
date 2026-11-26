package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kumoSleeping/iridium/internal/api"
)

// httpRig is a server on the TCP address, no public origin: every request is the host's.
type httpRig struct {
	t    *testing.T
	base string
	work string
}

func newHTTPRig(t *testing.T) *httpRig {
	s, _, work := setup(t)
	tcp, unix, err := s.Listen()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Serve(ctx, tcp, unix); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return &httpRig{t: t, base: "http://" + tcp.Addr().String(), work: work}
}

func (p *httpRig) do(method, path string, body, out any) int {
	p.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = strings.NewReader(string(b))
	}
	req, _ := http.NewRequest(method, p.base+path, r)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil {
		json.Unmarshal(b, out)
	}
	if resp.StatusCode != 200 {
		p.t.Logf("%s %s: %d %s", method, path, resp.StatusCode, b)
	}
	return resp.StatusCode
}

// The queue over HTTP.
func TestQueueRoutes(t *testing.T) {
	t.Parallel()
	p := newHTTPRig(t)
	created, _ := createAt(p.t, http.DefaultClient, p.base, api.NewSession{Agent: api.AgentClaude, Cwd: p.work, Text: "one HOLD"})
	id, sentInput := created, ""
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		var s api.SessionState
		p.do("GET", "/v1/internal/sessions/"+id, nil, &s)
		if s.Status == api.StatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no turn")
		}
	}
	// A command waiting in the queue for the turn's end (words would join the turn): changed
	// (message.update), not one that is not there (message.delete).
	if _, sent := sendAt(p.t, http.DefaultClient, p.base, id, api.Send{Text: "/two"}); !sent.Queued {
		t.Fatalf("not queued: %+v", sent)
	} else {
		var edited string
		if code := satAt(p.t, http.DefaultClient, p.base, "message.update", map[string]any{"channel_id": id, "message_id": sent.Input, "content": "/two!"}, &edited); code != 200 {
			t.Errorf("edit: %d %s", code, edited)
		}
		var s api.SessionState
		if p.do("GET", "/v1/internal/sessions/"+id, nil, &s); len(s.Queue) != 1 || s.Queue[0].Text != "/two!" {
			t.Errorf("edited: %+v", s.Queue)
		}
		if code := satAt(p.t, http.DefaultClient, p.base, "message.delete", map[string]any{"channel_id": id, "message_id": "nope"}, nil); code != 422 {
			t.Errorf("delete nothing: %d", code)
		}
		sentInput = sent.Input
	}
	if code := p.do("POST", "/v1/internal/sessions/"+id+"/queue/"+sentInput+"/steer", nil, nil); code != 200 {
		t.Errorf("steer: %d", code)
	}
	if code := p.do("PUT", "/v1/internal/sessions/"+id+"/mode", api.SetMode{Mode: "nope"}, nil); code != 400 {
		t.Errorf("bad mode: %d", code)
	}
	if code := p.do("POST", "/v1/internal/sessions/"+id+"/continue", nil, nil); code != 200 {
		t.Errorf("continue: %d", code)
	}
	var rules []api.AllowRule
	if code := p.do("GET", "/v1/internal/sessions/"+id+"/rules", nil, &rules); code != 200 || rules == nil {
		t.Errorf("rules: %d %v", code, rules)
	}

}

// An ask is answered by its card (internal/button): a card of no session, not an ask's, or an ask
// no longer open is not there.
func TestAnswerCard(t *testing.T) {
	t.Parallel()
	l := start(t)
	a := l.create(200, "socket", api.NewSession{Agent: api.AgentCodex, Cwd: l.work, Text: "hi"})
	for _, card := range []string{"ask:other:1", "pick:1", "ask:" + a + ":none"} {
		if code := l.answer("socket", api.CardAnswer{Card: card, Option: "allow"}); code != 422 {
			t.Errorf("%s: %d", card, code)
		}
	}
}
