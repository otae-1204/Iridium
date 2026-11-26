// Package deliver decides who a message said in a channel wakes (docs/channels.md "投递"): pure
// rules over the message's words, who said it and the channel's agents. It sends nothing.
package deliver

import (
	"slices"
	"strings"
	"unicode"

	"github.com/kumoSleeping/iridium/internal/api"
)

// MaxHops is how many times in a row agents (and the scripts they set off) may wake one another
// since a person last spoke, by naming or by a subscription alike: a backstop, far past what
// work handed on among agents comes to, with a person away (kumo, 2026-11-26; 4 from 2026-11-23,
// which stopped a run of work one led). Reached, the channel says it is paused and a person may
// go on (继续).
const MaxHops = 30

// MaxIdle is how many times in a row agents may wake one another with turns of words alone, no
// tool called: talk going round (thanks back and forth, asking again), paused as MaxHops is.
const MaxIdle = 4

// Agent is an agent of the channel: the session it is, the nick it is called by, and its
// subscriptions (On): what of the messages that name nobody wakes it (Fires).
type Agent struct {
	Session string
	Nick    string
	On      []api.Trigger
}

// From is who said a message: Member, as a subscription's from names it ("agent:<session>", a login
// "<app>/<login>"); Session, an agent's. Quiet: it wakes only whom it names, nobody by a receive
// subscription (a script's words; an agent's words on its way, not its turn's last).
type From struct {
	Member  string
	Session string
	Quiet   bool
}

// Decision is what a message does. Wake: the sessions it is delivered to, in the channel's
// order; Named: the nicks it names (itself aside); Texts: for a session its on woke, the entries
// that did and have words for it (Trigger.Text).
type Decision struct {
	Wake  []string
	Named []string
	Texts map[string][]api.Trigger
}

// The two names that call more than one (the user, 2026-11-13): "@全体成员", everyone in the
// channel, and "@全体Agent", its agents. For the agents both are the same: each is woken as if
// named. No nick may be one of them.
const (
	AllMembers = "全体成员"
	AllAgents  = "全体Agent"
)

// Reserved: n is one of the names that call more than one, whatever its case.
func Reserved(n string) bool {
	return strings.EqualFold(n, AllMembers) || strings.EqualFold(n, AllAgents)
}

// Route decides for a message with words text, said by from. One that names agents (@nick,
// @全体…) wakes them alone; one that names nobody wakes the agents one of whose subscriptions
// it fires (Fires), unless it is quiet. Each is woken once, whatever woke it. The sender is never woken by its own words. How many in
// a row may pass a message on (MaxHops) is counted by the channel.
func Route(text string, from From, agents []Agent) Decision {
	var nicks []string
	for _, a := range agents {
		if a.Session != from.Session || from.Session == "" {
			nicks = append(nicks, a.Nick)
		}
	}
	var d Decision
	d.Named = Mentions(text, nicks)
	if all := Mentions(text, []string{AllMembers, AllAgents}); len(all) > 0 {
		d.Named = nicks // all of them, as if each were named
	}
	named := map[string]bool{}
	for _, n := range d.Named {
		named[strings.ToLower(n)] = true
	}
	for _, a := range agents {
		if from.Session != "" && a.Session == from.Session {
			continue
		}
		if len(d.Named) > 0 {
			if named[strings.ToLower(a.Nick)] {
				d.Wake = append(d.Wake, a.Session)
			}
		} else if !from.Quiet && from.Member != "" {
			fired, texts := false, []api.Trigger(nil)
			for _, t := range a.On {
				if _, ok := Fires(t, from.Member, text); ok {
					fired = true
					if t.Text != "" && !slices.ContainsFunc(texts, func(x api.Trigger) bool { return x.Text == t.Text }) {
						texts = append(texts, t)
					}
				}
			}
			if fired {
				d.Wake = append(d.Wake, a.Session)
				if len(texts) > 0 {
					if d.Texts == nil {
						d.Texts = map[string][]api.Trigger{}
					}
					d.Texts[a.Session] = texts
				}
			}
		}
	}
	return d
}

// Fires: subscription t is set off by a message of sender (a login "<app>/<login>", an agent
// "agent:<session>": its turn's last words, a script "script:<name>") with words text. A message
// subscription: the sender is one of its from (Receives; none: anyone) and the words have one
// of its words (case not minded; none: any). A command one: a person's "/<command>", args the
// words after it.
func Fires(t api.Trigger, sender, text string) (args string, ok bool) {
	switch {
	case t.Message != nil:
		if len(t.Message.From) > 0 && !Receives(t.Message.From, sender) {
			return "", false
		}
		if len(t.Message.Words) == 0 {
			return "", true
		}
		low := strings.ToLower(text)
		return "", slices.ContainsFunc(t.Message.Words, func(w string) bool { return strings.Contains(low, strings.ToLower(w)) })
	case t.Command != "":
		if strings.HasPrefix(sender, "agent:") {
			return "", false
		}
		return CommandArgs(strings.TrimSpace(text), t.Command)
	}
	return "", false
}

// CommandArgs: body is "/<command>" alone or with words after a space; args, those words.
func CommandArgs(body, command string) (string, bool) {
	rest, ok := strings.CutPrefix(body, "/")
	if !ok || command == "" {
		return "", false
	}
	word, args, _ := strings.Cut(rest, " ")
	word, more, _ := strings.Cut(word, "\n")
	if !strings.EqualFold(word, command) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSpace(more) + " " + args), true
}

// Reacts are the agents a reaction by from is given (docs/channels.md "表情"): those with a
// message subscription of from's words, of no words in particular; never from itself. A reaction names
// nobody: @ in an emoji calls no one.
func Reacts(from From, agents []Agent) []string {
	var out []string
	for _, a := range agents {
		if from.Session != "" && a.Session == from.Session {
			continue
		}
		if from.Member != "" && slices.ContainsFunc(a.On, func(t api.Trigger) bool {
			return t.Message != nil && len(t.Message.Words) == 0 && (len(t.Message.From) == 0 || Receives(t.Message.From, from.Member))
		}) {
			out = append(out, a.Session)
		}
	}
	return out
}

// Receives: set (a subscription's from) has sender ("agent:<session>", a login
// "<app>/<login>"): "*" is every person, not an agent.
func Receives(set []string, sender string) bool {
	agent := strings.HasPrefix(sender, "agent:")
	for _, e := range set {
		switch {
		case e == sender:
			return true
		case agent:
		case e == "*":
			return true
		case strings.HasSuffix(e, "/*") && strings.HasPrefix(sender, strings.TrimSuffix(e, "*")):
			return true
		}
	}
	return false
}

// ValidNick: a nick is letters, digits, "_" and "-", at most 32 of them, so that "@nick"
// stands out of any words; not one of the names that call more than one.
func ValidNick(n string) bool {
	if n == "" || len([]rune(n)) > 32 || Reserved(n) {
		return false
	}
	for _, r := range n {
		if !nickRune(r) {
			return false
		}
	}
	return true
}

// NickFrom is a nick made of name (a session's name): its letters, digits, _ and -, at most
// 32; "" if that is no nick.
func NickFrom(name string) string {
	var b strings.Builder
	n := 0
	for _, r := range name {
		if nickRune(r) && n < 32 {
			b.WriteRune(r)
			n++
		}
	}
	if !ValidNick(b.String()) {
		return ""
	}
	return b.String()
}

func nickRune(r rune) bool {
	return r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Mentions are the nicks (of those given, as given, each once, in the order named) that text
// calls: "@nick" standing by itself (not inside a word, an address or a longer name), outside
// code (a fenced block, an inline span) and outside quoted lines ("> "). Case does not matter.
func Mentions(text string, nicks []string) []string {
	if len(nicks) == 0 || !strings.Contains(text, "@") {
		return nil
	}
	by := map[string]string{}
	for _, n := range nicks {
		by[strings.ToLower(n)] = n
	}
	var out []string
	seen := map[string]bool{}
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced || strings.HasPrefix(t, ">") {
			continue
		}
		rs := []rune(line)
		code := false
		for i := 0; i < len(rs); i++ {
			switch {
			case rs[i] == '`':
				code = !code
			case rs[i] == '@' && !code && (i == 0 || !nickRune(rs[i-1]) && rs[i-1] != '@' && rs[i-1] != '/' && rs[i-1] != '.'):
				j := i + 1
				for j < len(rs) && nickRune(rs[j]) {
					j++
				}
				if n, ok := by[strings.ToLower(string(rs[i+1:j]))]; ok && !seen[n] {
					seen[n] = true
					out = append(out, n)
				}
				i = j - 1
			}
		}
	}
	return out
}
