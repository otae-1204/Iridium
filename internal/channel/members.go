package channel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/deliver"
	"github.com/kumoSleeping/iridium/internal/store"
)

var (
	ErrNick     = errors.New("a nick is letters, digits, _ and -, at most 32, and one agent's alone")
	ErrNoMember = errors.New("no such member")
	ErrMember   = errors.New("already in the channel")
	ErrChannel  = errors.New("no such channel")
	ErrFull     = errors.New("the channel has as many agents as it may")
)

// written makes channel's default members rows of their own, once someone changes them:
// everyone and, in a session's channel, the session it is, which has seen everything said so
// far, keeps receiving everything and speaks directly. base names an agent's harness, for its
// nick.
func (s *Service) written(channel string, base func(session string) string) error {
	rows, err := s.st.ChannelMembers(channel)
	if err != nil || len(rows) > 0 {
		return err
	}
	_, own := s.Own(channel)
	if !own {
		s.Touch(channel)
	}
	end, err := s.st.ChannelEnd(channel)
	if err != nil {
		return err
	}
	defer s.changed()
	if err := s.st.PutChannelMember(store.ChannelMember{Channel: channel, Member: Everyone, Triggers: PersonRow}); err != nil {
		return err
	}
	if own {
		return nil
	}
	if _, err := s.nickOf(channel, base); err != nil {
		return err
	}
	return s.st.PutChannelMember(store.ChannelMember{Channel: channel, Member: store.AgentMember(channel), Triggers: EncodeTriggers(EveryoneOn), Send: store.SendDirect, Seen: end})
}

// nickOf is session's nick: the one it has, else one made from its harness's name (base,
// base2, base3…: the first no agent has) and kept.
func (s *Service) nickOf(session string, base func(session string) string) (string, error) {
	if n := s.Nick(session); n != "" {
		return n, nil
	}
	taken := map[string]bool{}
	for _, n := range s.members().nicks {
		taken[strings.ToLower(n)] = true
	}
	var nick string
	if s.Numbered {
		stem := strings.Map(func(r rune) rune {
			if deliver.ValidNick(string(r)) {
				return r
			}
			return -1
		}, strings.ToLower(base(session)))
		if stem == "" {
			stem = "agent"
		}
		nick = stem
		for i := 2; taken[nick]; i++ {
			nick = fmt.Sprintf("%s%d", stem, i)
		}
	} else {
		nick = DefaultNick(base(session), session, func(n string) bool { return taken[strings.ToLower(n)] })
	}
	defer s.changed()
	return nick, s.st.SetNick(session, nick)
}

// stems are the names a default nick begins with, by harness.
var stems = map[string]string{"claude": "Claude", "codex": "Codex", "pi": "Pi", "dsh": "DeepSeek"}

// defaultNick is what DefaultNick makes: a harness's name, "-", hex digits; agy's ("Agent-…"
// here) also as named by its Gemini model's code name (Neon-d04a, Hydrogen-…), so that agy is
// asked to name itself too (kumo, 2026-11-26). A role's name with its session's digits after it
// (NickWithID) is a name chosen, not one of these.
var defaultNick = regexp.MustCompile(`^(Agent|Claude|Codex|DeepSeek|Pi|Agy|Gemini|Argon|Hydrogen|Neon)-[0-9a-f]{4,}$`)

// DefaultNick is the nick an agent goes by till it gives itself one (kumo, 2026-11-16: not
// claude2): its harness's name and four hex digits of its session's id (Claude-b134), more
// digits while taken says it is taken.
func DefaultNick(harness, session string, taken func(string) bool) string {
	stem := stems[harness]
	if stem == "" {
		stem = "Agent"
	}
	return NickWithID(stem, session, taken)
}

// NickWithID is base, "-" and four hex digits of session's id (克劳德大小姐-e444), more digits
// while taken says it is taken: the nick of a session whose name another has (kumo,
// 2026-11-17: not 克劳德大小姐2). base is cut to fit a nick's 32.
func NickWithID(base, session string, taken func(string) bool) string {
	digits := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			return r
		}
		return -1
	}, strings.ToLower(strings.TrimPrefix(session, "session-"))) // dsh's ids begin so
	with := func(tail string) string {
		b := []rune(base)
		if n := 32 - len([]rune(tail)) - 1; len(b) > n {
			b = b[:max(n, 0)]
		}
		return string(b) + "-" + tail
	}
	for n := 4; n <= len(digits); n++ {
		if nick := with(digits[:n]); !taken(nick) {
			return nick
		}
	}
	// Every digit of its id taken, or an id of few: digits of hashes of it, never a number.
	for i := 0; ; i++ {
		sum := sha256.Sum256([]byte(session + "\x00" + strconv.Itoa(i)))
		if nick := with(hex.EncodeToString(sum[:])[:min(4+i/8, 64)]); !taken(nick) {
			return nick
		}
	}
}

// IsDefaultNick: nick is one DefaultNick gave, not one the agent or a person chose.
func IsDefaultNick(nick string) bool { return defaultNick.MatchString(nick) }

// NickTaken: an agent goes by nick (any case).
func (s *Service) NickTaken(nick string) bool {
	for _, n := range s.members().nicks {
		if strings.EqualFold(n, nick) {
			return true
		}
	}
	return false
}

// SetNick makes nick what session's agent is called by, in every channel.
func (s *Service) SetNick(session, nick string) error {
	if !deliver.ValidNick(nick) {
		return ErrNick
	}
	defer s.changed()
	if err := s.st.SetNick(session, nick); errors.Is(err, store.ErrNickTaken) {
		return ErrNick
	} else if err != nil {
		return err
	}
	return nil
}

// Join puts session's agent in channel, its subscriptions on (none: only what names it): a
// subscription. There it sees what is said and is called by what names it; it has seen nothing
// of the channel yet. nick, if any, becomes what it is called by. base names a session's
// harness. full: given the channel's agents now, there is no room for another (nil: any
// number). One at a time, so that two joining at once do not both find room; a join that fails
// leaves the nick as it was.
func (s *Service) Join(channel, session string, on []api.Trigger, nick string, base func(session string) string, full func(agents []string) bool) (Member, error) {
	s.jmu.Lock()
	defer s.jmu.Unlock()
	if s.Speaks(channel, session) {
		return Member{}, ErrMember
	}
	on, err := CleanOn(on, store.AgentMember(session))
	if err != nil {
		return Member{}, err
	}
	if nick != "" && !deliver.ValidNick(nick) {
		return Member{}, ErrNick
	}
	if full != nil && full(s.Agents(channel)) {
		return Member{}, ErrFull
	}
	if err := s.written(channel, base); err != nil {
		return Member{}, err
	}
	was := s.Nick(session)
	if nick != "" {
		if err := s.SetNick(session, nick); err != nil {
			return Member{}, err
		}
	} else if _, err := s.nickOf(session, base); err != nil {
		return Member{}, err
	}
	row := store.ChannelMember{Channel: channel, Member: store.AgentMember(session), Triggers: EncodeTriggers(on), Send: store.SendIR}
	err = s.st.PutChannelMember(row)
	s.changed()
	if err != nil {
		if was != "" && was != s.Nick(session) {
			_ = s.SetNick(session, was)
		}
		return Member{}, err
	}
	return Member{ChannelMember: row, Session: session, Nick: s.Nick(session)}, nil
}

// Moved: session next goes on in session old's place (a session cleared): old's own channel's
// members, the channels old is in, its nick and its face are next's.
func (s *Service) Moved(old, next string) error {
	s.jmu.Lock()
	defer s.jmu.Unlock()
	defer s.changed()
	return s.st.MoveAgent(old, next)
}

// Leave takes member out of channel. The session a channel is its own stays in it.
func (s *Service) Leave(channel, member string) error {
	if _, own := s.Own(channel); !own && member == store.AgentMember(channel) {
		return fmt.Errorf("%w: the channel is this session's", ErrNoMember)
	}
	defer s.changed()
	ok, err := s.st.RemoveChannelMember(channel, member)
	if err == nil && !ok {
		err = ErrNoMember
	}
	return err
}

// Change sets a member's nick (an agent's, in every channel) and an agent's subscriptions (nil:
// as they are).
func (s *Service) Change(channel, member string, nick *string, on *[]api.Trigger, base func(session string) string) (Member, error) {
	if err := s.written(channel, base); err != nil {
		return Member{}, err
	}
	ms, err := s.Members(channel)
	if err != nil {
		return Member{}, err
	}
	for _, m := range ms {
		if m.Member != member {
			continue
		}
		if nick != nil {
			if !m.Agent() {
				return Member{}, ErrNick
			}
			if err := s.SetNick(m.Session, *nick); err != nil {
				return Member{}, err
			}
			m.Nick = *nick
		}
		if on != nil {
			if !m.Agent() {
				return Member{}, fmt.Errorf("%w: a person subscribes to nothing", ErrOn)
			}
			set, err := CleanOn(*on, m.Member)
			if err != nil {
				return Member{}, err
			}
			m.Triggers = EncodeTriggers(set)
		}
		defer s.changed()
		return m, s.st.PutChannelMember(m.ChannelMember)
	}
	return Member{}, ErrNoMember
}

// Create makes a channel of its own: no session's, listed in cwd at machine. Its people are
// everyone until they are set (the server sets a new one's as it makes it); it has no agent
// until one joins.
func (s *Service) Create(c store.Channel) error {
	if c.ID == "" {
		return ErrChannel
	}
	defer s.changed()
	return s.st.PutChannel(c)
}

// Rename names a channel of its own.
func (s *Service) Rename(id, name string) error {
	c, ok := s.Own(id)
	if !ok {
		return ErrChannel
	}
	c.Name = name
	defer s.changed()
	return s.st.PutChannel(c)
}

// Remove takes a channel of its own away, its members with it; what was said in it stays.
func (s *Service) Remove(id string) error {
	if _, ok := s.Own(id); !ok {
		return ErrChannel
	}
	defer s.changed()
	return s.st.DeleteChannel(id)
}

// Seen moves session's read position in its channel up to seq.
func (s *Service) Seen(channel, session string, seq int64) {
	defer s.changed()
	if err := s.st.SetChannelSeen(channel, store.AgentMember(session), seq); err != nil {
		s.logf("channel %s: %s read to %d: %v", channel, session, seq, err)
	}
}

// Unseen moves session's read position in channel back to seq: what it was to be given after
// that never reached it.
func (s *Service) Unseen(channel, session string, seq int64) {
	defer s.changed()
	if err := s.st.RewindChannelSeen(channel, store.AgentMember(session), seq); err != nil {
		s.logf("channel %s: %s back to %d: %v", channel, session, seq, err)
	}
}

// Between are channel's messages with numbers above after and below before, the latest limit
// of them, in order.
func (s *Service) Between(channel string, after, before int64, limit int) ([]Message, error) {
	rows, err := s.st.ChannelMessages(channel, before, -1, limit)
	if err != nil {
		return nil, err
	}
	var out []Message
	for _, r := range rows {
		if r.Seq > after {
			out = append(out, read(r))
		}
	}
	return out, nil
}

// ---- the people ----

// PersonMember is the member a login of an app is; AppMember, all of an app's logins.
func PersonMember(app, login string) string { return app + "/" + login }
func AppMember(app string) string           { return app + "/*" }

// validPeople: m names people: everyone, an app's logins, or one login.
func validPeople(m string) bool {
	if m == Everyone {
		return true
	}
	app, login, ok := strings.Cut(m, "/")
	return ok && app != "" && login != "" && !strings.HasPrefix(m, "agent:") && !strings.ContainsAny(m, " \n")
}

// People are the people of channel as written: Everyone, apps' logins ("<app>/*"), logins
// ("<app>/<login>").
func (s *Service) People(channel string) []string {
	ms, _ := s.Members(channel)
	var out []string
	for _, m := range ms {
		if !m.Agent() {
			out = append(out, m.Member)
		}
	}
	return out
}

// Has: login of app is one of channel's people, so may speak in it, answer its agents and
// change its members. Whoever is not still reads it.
func (s *Service) Has(channel, app, login string) bool {
	for _, m := range s.People(channel) {
		if m == Everyone || m == AppMember(app) || (login != "" && m == PersonMember(app, login)) {
			return true
		}
	}
	return false
}

// SetPeople makes people the people of channel, in place of those it had; none is everyone.
func (s *Service) SetPeople(channel string, people []string, base func(session string) string) error {
	if len(people) == 0 {
		people = []string{Everyone}
	}
	for _, p := range people {
		if !validPeople(p) {
			return fmt.Errorf("%w: %q", ErrPeople, p)
		}
	}
	if err := s.written(channel, base); err != nil {
		return err
	}
	defer s.changed()
	for _, old := range s.People(channel) {
		if !slices.Contains(people, old) {
			if _, err := s.st.RemoveChannelMember(channel, old); err != nil {
				return err
			}
		}
	}
	for _, p := range people {
		if slices.Contains(people, Everyone) && p != Everyone {
			continue // everyone: the rest says nothing more
		}
		if err := s.st.PutChannelMember(store.ChannelMember{Channel: channel, Member: p, Triggers: PersonRow}); err != nil {
			return err
		}
	}
	return nil
}

// ErrPeople: not a way to name people.
var ErrPeople = errors.New("people: \"*\", \"<app>/*\" or \"<app>/<login>\"")
