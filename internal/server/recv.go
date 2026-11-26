package server

import (
	"fmt"
	"strings"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/channel"
	"github.com/kumoSleeping/iridium/internal/gate"
	"github.com/kumoSleeping/iridium/internal/store"
)

// senderWords names an entry of an on's from: zh in Chinese for a line of the channel, else in
// English as the agents are told it.
func (s *Server) senderWords(e string, zh bool) string {
	switch {
	case e == channel.Everyone:
		return cond(zh, "全体成员", "any member")
	case strings.HasPrefix(e, "agent:"):
		id, _ := store.SessionOfMember(e)
		if n := s.ch.Nick(id); n != "" {
			return "@" + n
		}
		return e
	case strings.HasPrefix(e, scriptMemberPrefix):
		return "「" + strings.TrimPrefix(e, scriptMemberPrefix) + "」"
	case e == channel.AppMember(api.WebApp):
		return "Ir-Web"
	case e == channel.AppMember(gate.HostID):
		return "Iridium"
	case strings.HasSuffix(e, "/*"):
		app := strings.TrimSuffix(e, "/*")
		return cmpOr(s.Tag(app), app)
	}
	for _, p := range s.persons() {
		if p.member == e {
			return p.name
		}
	}
	return e
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// onWords says subscriptions as a line of the channel says them, a sentence each, as the page
// does (web/src/chat/channel/scripts.ts triggerLine): 被 @ 时；全体成员说话时；成员加入时 → 欢迎新人.
func (s *Server) onWords(on []api.Trigger) string {
	events := map[string]string{"join": "成员加入", "leave": "成员移出", "rename": "改名", "wake": "唤醒变更", "error": "出错", "ask": "提问"}
	out := []string{"被 @ 时"}
	for _, t := range on {
		var w string
		switch {
		case t.Command != "":
			w = "发送 /" + t.Command + " 时"
		case t.Cron != "":
			w = "定时 " + t.Cron
		case len(t.Event) > 0:
			var evs []string
			for _, e := range t.Event {
				evs = append(evs, cmpOr(events[channel.EventKind(e)], e))
			}
			w = strings.Join(evs, "或") + "时"
		case t.Message != nil:
			var from []string
			for _, e := range t.Message.From {
				from = append(from, s.senderWords(e, true))
			}
			who := strings.Join(from, "、")
			switch {
			case len(t.Message.Words) > 0:
				w = who + "说到「" + strings.Join(t.Message.Words, "」或「") + "」时"
			case who != "":
				w = who + "说话时"
			default:
				w = "有消息时"
			}
		}
		if t.Text != "" {
			w += " → " + t.Text
		}
		out = append(out, w)
	}
	return strings.Join(out, "；")
}

// eventWords are the events as the agents are told them.
var eventWords = map[string]string{"join": "a member joins", "leave": "a member leaves", "rename": "an agent is renamed", "wake": "wake rules change", "error": "an agent fails", "ask": "an agent asks"}

// ruleWords says one entry of an on as the agents are told it, without its words:
// any member saying "小克", what the panel or Iridium says, a member joins, at "0 9 * * *", /standup.
func (s *Server) ruleWords(t api.Trigger) string {
	switch {
	case t.Command != "":
		return "/" + t.Command
	case t.Cron != "":
		return fmt.Sprintf("at %q", t.Cron)
	case len(t.Event) > 0:
		var evs []string
		for _, e := range t.Event {
			evs = append(evs, cmpOr(eventWords[channel.EventKind(e)], e))
		}
		return strings.Join(evs, " or ")
	case t.Message != nil:
		var from []string
		for _, e := range t.Message.From {
			from = append(from, s.senderWords(e, false))
		}
		who := strings.Join(from, " or ")
		if len(t.Message.Words) > 0 {
			return cmpOr(who, "anyone") + fmt.Sprintf(" saying %q", strings.Join(t.Message.Words, `" or "`))
		}
		if who == "" {
			return "any message"
		}
		return "what " + who + " says"
	}
	return ""
}

// triggerWords says an on as the agents are told it, each entry with what it asks: "" for none.
func (s *Server) triggerWords(on []api.Trigger) string {
	var out []string
	for _, t := range on {
		w := s.ruleWords(t)
		if t.Text != "" {
			w += " → " + t.Text
		}
		out = append(out, w)
	}
	return strings.Join(out, "; ")
}
