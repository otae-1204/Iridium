// Package irhelp is ir's help: what ir --help prints, and its agent part. A hosted agent's
// system prompt (assemble.System) says the same commands in its own fragments, with
// ir api help's topics from here.
package irhelp

import "strings"

// Help is ir --help: at most 20 lines (plan "ir"). The agent part is before Admin.
const Help = `ir: the host's Iridium on the command line (over its unix socket).
  ir api [METHOD] <path|satori.method> [k=v | k:=json | k==query]... [--wait]   Iridium's API as it is: ir api help
  ir run [--secret name,…] -- command …   a command line with secrets in it ({{name}}, {{file:name}}, env vars); their values masked in its output
Admin, a person at a terminal:
  ir start [--headless] [--no-server] [--join <address>]... | stop | status   ir as this computer's service (at login too), its window unless --headless | stopped and taken away | what runs here
  ir server on|off | join <address> [--no-shell] [--agents a,b] [--roots d,…] [--proxy] | leave <address>   this computer's own Iridium on or off | joined to another Iridium, kept | not any more
  ir serve [--no-server] [--join <address>]...   the same in this terminal, till it closes
  ir public [--host <host>] | off | pair [<address>] --name n --env file:VAR   go public through Cloudflare | an app's TOKEN and login by pairing, into a file; no address: here (--help on each)
`

// Agent is Help's agent part: the commands an agent uses, without Admin.
func Agent() string {
	h, _, _ := strings.Cut(Help, "Admin")
	return h
}

// Remote is the agent part for a session hosted at another computer (docs/exec.md "远端的 ir"):
// its ir reaches the Iridium over HTTPS with the session's own TOKEN, as that computer's app. The
// session commands as Help has them; a folder is this computer's; the host's files, command lines
// and secrets are not reached from here.
func Remote() string {
	var b strings.Builder
	b.WriteString("ir: the Iridium hosting this session, reached over HTTPS as this computer's app. A folder is this computer's.\n")
	for _, l := range strings.Split(Agent(), "\n") {
		if strings.HasPrefix(l, "  ir api ") {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("  Not from here: ir run, the routes marked socket only, the admin commands; ask a session on the host.\n")
	return b.String()
}
