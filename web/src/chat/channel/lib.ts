// A channel of several agents (docs/channels.md), what the page reads of it, pure for tests: its
// messages as the log gives them (Satori's message.list: only what was said; rows.ts puts them in
// order and under their speakers); the @nick that calls an agent, found in words as the server finds it
// (internal/deliver Mentions) and put into the composer; the agents listed under a channel's row;
// a channel's lamp.
import type { ChannelMessage } from "../../api/client";
import type { Agent, Device, FileRef, Session, Status, Trigger, Ask } from "../../api/types.gen";
import { blockOf } from "../../satori";
import { onOf, recvOf, scriptOfUser } from "./scripts";

/** Who said a message: an agent of the channel (its session, the nick it is called by, what it
 *  ran as), a person (their tag), a script of the channel's IR (its name), or Iridium itself. */
export type Author = { kind: "agent"; session: string; nick: string; name: string } | { kind: "person"; id: string; name: string; device?: Device } | { kind: "script"; name: string } | { kind: "iridium" };

/** A message of a channel: n, its number there. kind: as its event said it (user, progress, reply,
 *  ask, error, note); the log itself does not say. */
/** Where an agent's message is from in its own record (docs/channels.md "指向"): said straight
 *  out, the block; with ir, the lines from (where it was called) to to (where it spoke). */
export type Src = { session: string; block?: string; from?: number; to?: number };

/** input: a person's message, the input it was sent as (a session's record has it by that id);
 *  an agent's sent with ir, the same. A merged forward in its text is drawn apart (forward.ts
 *  msgWords). writing: an agent's, still being written (docs/next/ui.md: told as it begins and as
 *  it grows); work: what the agent did after it (<iridium:work>); a message of kind "work" is
 *  that alone, no words. did: what it did before it, as older messages say (<iridium:activity>). */
export type Msg = { n: number; text: string; files: FileRef[]; time?: number; author: Author; kind?: string; did?: Did; work?: Work; writing?: boolean; event?: ChEvent; src?: Src; quote?: number; input?: string; ask?: Ask; card?: string };

/** What an agent did after a message of its turn (<iridium:work>, internal/satori/live.go
 *  workElement): steps, its calls so far; running, while it goes on; tool, the latest call's name;
 *  at, when the first began (ms); ms, how long it all took, once done; from, to: the lines of the
 *  agent's own record it spans (to: none while it runs). */
export type Work = { steps: number; running: boolean; tool?: string; at?: number; ms?: number; from: number; to?: number; agents?: number };

/** Something that happened to a channel, a line of Iridium's own in it (internal/channel Event):
 *  join, leave (nick), rename (from, nick), on (nick, on: its subscriptions now), recv (before
 *  2026-11-23: nick, recv: its receive set; before 2026-11-17 "all" or "at"), script (name, what: add, remove, enabled, disabled or the fields changed;
 *  from: its old name), people (added, removed: member
 *  strings; everyone), undelivered (seq, nick), stopped (seq), silent (seq, nick: called for it,
 *  it said nothing in the channel; reason: why, its turn ending in an error), profile (a card
 *  changed: member's, by nick, or the channel's own with no member; what: the fields, "name",
 *  "nick", "persona", "avatar"; older lines "intro", "background" too, not drawn). by: who did it, by
 *  name (the host's as Iridium, however it was kept: 主机 before 2026-11-15). */
export type ChEvent = { type: string; member?: string; nick?: string; name?: string; from?: string; recv?: string[] | string; on?: Trigger[]; added?: string[]; removed?: string[]; everyone?: boolean; seq?: number; count?: number; reason?: string; by?: string; what?: string[] };

/** A piece of an event's line: words, an agent by its nick, people by their member strings, a
 *  receive set (named as the page names its entries). */
export type EventPart = string | { nick: string } | { people: string[] } | { recv: string[] } | { on: Trigger[] };

/** What a script's line says, by what was done to it (ChEvent what; renamed: from is set). */
export type ScriptDone = "add" | "remove" | "enabled" | "disabled" | "changed" | "renamed";

/** The line's words by event (i18n: channel.event.*), their slots in braces: {nick} {from}
 *  {recv} {people} {seq} {reason} {by} {script} {old}; reasons: the server's usual reasons (余额不足…) in them. */
export type EventWords = Record<"join" | "leave" | "rename" | "recv" | "on" | "wake" | "everyone" | "added" | "removed" | "undelivered" | "stopped" | "silent" | "silentWhy" | "by" | "sep", string> & {
  /** A script added, removed, turned on or off, changed, renamed: each with who did it in its words. */
  script?: Record<ScriptDone, string>;
  reasons?: Record<string, string>;
  /** A card changed: a member's, the channel's; each with who did it in its words, and without. */
  profile?: string;
  profileChannel?: string;
  profileNoBy?: string;
  profileChannelNoBy?: string;
  /** The fields of a card by name ({what}), and what stands between them. */
  what?: Record<string, string>;
  whatSep?: string;
};

/** An event as the pieces of its line, in the page's words; none for one not known (its message's
 *  own text is shown then). */
export function eventParts(e: ChEvent, w: EventWords): EventPart[] | undefined {
  const fill = (tpl: string): EventPart[] =>
    tpl
      .split(/(\{[a-z]+\})/)
      .filter(Boolean)
      .map((x): EventPart => {
        switch (x) {
          case "{nick}":
            return { nick: e.nick ?? "" };
          case "{from}":
            return `@${e.from ?? ""}`;
          case "{recv}":
            return { recv: recvOf(e.recv) };
          case "{on}":
            return { on: onOf(e.on) };
          case "{script}":
            return e.name ?? "";
          case "{old}":
            return e.from ?? "";
          case "{seq}":
            return String(e.seq ?? "");
          case "{by}":
            return e.by === "主机" || e.by === "本机" ? "Iridium" : (e.by ?? "");
          case "{reason}":
            return w.reasons?.[e.reason ?? ""] ?? e.reason ?? "";
          case "{added}":
            return { people: e.added ?? [] };
          case "{removed}":
            return { people: e.removed ?? [] };
          case "{what}":
            return (e.what ?? []).map((x) => w.what?.[x] ?? x).join(w.whatSep ?? ", ");
        }
        return x;
      });
  let out: EventPart[];
  switch (e.type) {
    case "join":
    case "leave":
    case "rename":
    case "recv":
    case "on":
    case "wake":
    case "undelivered":
    case "stopped":
      out = fill(w[e.type]);
      break;
    case "silent":
      out = fill(e.reason ? w.silentWhy : w.silent);
      break;
    case "people": {
      const bits = [e.everyone ? fill(w.everyone) : [], e.added?.length ? fill(w.added) : [], e.removed?.length ? fill(w.removed) : []].filter((b) => b.length);
      if (!bits.length) return undefined;
      out = bits.flatMap((b, i) => (i ? [w.sep, ...b] : b));
      break;
    }
    case "script": {
      const tpl = w.script?.[scriptDone(e)];
      if (!tpl || !e.name) return undefined;
      out = fill(tpl);
      break;
    }
    case "profile": {
      // Fields cards no longer have (an intro, a background: kumo, 2026-11-25) left out of an
      // older line; one of nothing else, not drawn.
      if (w.what) e = { ...e, what: (e.what ?? []).filter((x) => x in w.what!) };
      // Who did it is in the line's own words, not after it.
      const tpl = e.member ? (e.by ? w.profile : w.profileNoBy) : e.by ? w.profileChannel : w.profileChannelNoBy;
      if (!tpl || !e.what?.length) return undefined;
      out = fill(tpl);
      break;
    }
    default:
      return undefined;
  }
  if (e.by && e.type !== "profile" && e.type !== "script") out.push(...fill(w.by));
  // Words side by side are one piece; none by whom, no space before the rest.
  const one = out.reduce<EventPart[]>((all, p) => (typeof p === "string" && typeof all.at(-1) === "string" ? [...all.slice(0, -1), (all.at(-1) as string) + p] : [...all, p]), []);
  if (typeof one[0] === "string") one[0] = one[0].trimStart();
  return one.filter((p) => p !== "");
}

/** What a script's line says was done to it: renamed (its old name given) before the rest. */
function scriptDone(e: ChEvent): ScriptDone {
  const what = e.what ?? [];
  if (e.from) return "renamed";
  for (const k of ["add", "remove", "enabled", "disabled"] as const) if (what.includes(k)) return k;
  return "changed";
}

/** A person's entry by name: an app's all its name, a login its own; not known, as written. */
export function personName(member: string, apps: { id: string; name: string }[], logins: { app: string; id: string; name: string }[]): string {
  const i = member.indexOf("/");
  const app = member.slice(0, i), login = member.slice(i + 1);
  if (i < 0) return member;
  if (login === "*") return apps.find((a) => a.id === app)?.name ?? member;
  return logins.find((l) => l.app === app && l.id === login)?.name ?? member;
}

/** What an agent did before it said a message (docs/channels.md "活动"): its tool calls counted,
 *  the programs its commands began with (cmds, "go 4, git 2"), the files it changed. */
export type Did = { bash: number; failed: number; cmds: string; read: number; web: number; agents: number; other: number; files: number; add: number; del: number; paths: { path: string; add: number; del: number }[] };

const ATTR = /([a-z]+)="([^"]*)"/g;
const unesc = (s: string) => s.replace(/&quot;/g, '"').replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&amp;/g, "&");

/** The <iridium:activity> of a message's content, if it has one. */
export function didOf(content: string): Did | undefined {
  const at = content.lastIndexOf("<iridium:activity");
  if (at < 0) return undefined;
  const rest = content.slice(at);
  const head = rest.slice(0, rest.indexOf(">") + 1);
  const a: Record<string, string> = {};
  for (const m of head.matchAll(ATTR)) a[m[1]!] = unesc(m[2]!);
  const num = (k: string) => Number(a[k] ?? 0) || 0;
  const paths = [...rest.matchAll(/<iridium:file ([^>]*)\/>/g)].map((f) => {
    const fa: Record<string, string> = {};
    for (const m of f[1]!.matchAll(ATTR)) fa[m[1]!] = unesc(m[2]!);
    return { path: fa.path ?? "", add: Number(fa.add ?? 0) || 0, del: Number(fa.del ?? 0) || 0 };
  });
  return { bash: num("bash"), failed: num("failed"), cmds: a.cmds ?? "", read: num("read"), web: num("search") + num("fetch"), agents: num("agents"), other: num("other"), files: num("files"), add: num("add"), del: num("del"), paths };
}

/** The <iridium:work> of a message's content, if it has one. */
export function workIn(content: string): Work | undefined {
  const all = [...content.matchAll(/<iridium:work\b[^>]*>/g)];
  const head = all.at(-1)?.[0];
  if (!head) return undefined;
  const a: Record<string, string> = {};
  for (const m of head.matchAll(ATTR)) a[m[1]!] = unesc(m[2]!);
  const num = (k: string) => Number(a[k] ?? 0) || 0;
  return { steps: num("steps"), running: a.running === "true", from: num("from"), ...(a.tool ? { tool: a.tool } : {}), ...(num("at") ? { at: num("at") } : {}), ...(num("ms") ? { ms: num("ms") } : {}), ...(num("agents") ? { agents: num("agents") } : {}), ...(num("to") ? { to: num("to") } : {}) };
}

const AGENT = "agent:";

/** m of the log as the page draws it; none for one without a number. */
export function msgOf(m: ChannelMessage): Msg | undefined {
  const n = Number(m.id);
  if (!Number.isInteger(n) || n <= 0) return undefined;
  const b = blockOf({ id: m.id, content: m.content, created_at: m.created_at });
  const u = m.user;
  const script = scriptOfUser(u?.id, m._data?.script);
  const author: Author = script ? { kind: "script", name: script } : u?.id.startsWith(AGENT) ? { kind: "agent", session: u.id.slice(AGENT.length), nick: u.nick ?? "", name: u.name ?? "" } : !u || u.id === "iridium" ? { kind: "iridium" } : { kind: "person", id: u.id, name: u.name ?? "", ...(m._data?.device ? { device: m._data.device } : {}) };
  // The message it answers: Satori's <quote id/> at its start.
  const q = /^<quote id="(\d+)"\s*\/>/.exec(m.content);
  const did = author.kind === "agent" ? didOf(m.content) : undefined;
  const work = author.kind === "agent" ? workIn(m.content) : undefined;
  return { n, text: b.text ?? "", files: b.files ?? [], author, ...(q ? { quote: Number(q[1]) } : {}), ...(m.created_at ? { time: m.created_at } : {}), ...(m._data?.kind ? { kind: m._data.kind } : {}), ...(m._data?.input ? { input: m._data.input } : {}), ...(m._data?.ask ? { ask: m._data.ask as Ask } : {}), ...(m._data?.card ? { card: m._data.card } : {}), ...(did ? { did } : {}), ...(work ? { work } : {}), ...(m._data?.writing ? { writing: true } : {}), ...(m._data?.event && typeof m._data.event === "object" ? { event: m._data.event as ChEvent } : {}), ...(author.kind === "agent" && m._data?.session ? { src: { session: m._data.session, ...(m._data.block ? { block: m._data.block } : {}), ...(typeof m._data.from === "number" ? { from: m._data.from } : {}), ...(m._data.to ? { to: m._data.to } : {}) } } : {}) };
}

const NICK_RUNE = /[\p{L}\p{N}_-]/u;
const NICK = /^[\p{L}\p{N}_-]+$/u;

/** A nick: letters, digits, "_" and "-", at most 32 of them (internal/deliver ValidNick). */
export const validNick = (n: string) => NICK.test(n) && [...n].length <= 32;

/** The names that call more than one in a channel (internal/deliver AllMembers, AllAgents). */
export const ALL_MEMBERS = "全体成员";
export const ALL_AGENTS = "全体Agent";

/** Where words name one of nicks (an agent's nick, a person's or an app's name, the names of
 *  all): "@name" standing by itself (not inside a word, an address or a longer name), outside
 *  code (a fenced block, an inline span) and quoted lines; case does not matter. An agent's nick
 *  as the server routes by (internal/deliver Mentions), each place it is said; a person's name
 *  may have what a nick may not (a space, a dot): the longest that fits is the one named. */
export function nickSpans(text: string, nicks: string[]): { from: number; to: number; nick: string }[] {
  if (!nicks.length || !text.includes("@")) return [];
  const names = [...new Set(nicks.filter(Boolean))].map((n) => [n.toLowerCase(), n] as const).sort((a, b) => b[0].length - a[0].length);
  const out: { from: number; to: number; nick: string }[] = [];
  let fenced = false;
  let at = 0;
  for (const line of text.split("\n")) {
    const start = at;
    at += line.length + 1;
    const t = line.trim();
    if (t.startsWith("```") || t.startsWith("~~~")) {
      fenced = !fenced;
      continue;
    }
    if (fenced || t.startsWith(">")) continue;
    const low = line.toLowerCase();
    let code = false;
    for (let i = 0; i < line.length; i++) {
      const c = line[i];
      if (c === "`") code = !code;
      if (code || c !== "@") continue;
      const before = [...line.slice(Math.max(0, i - 2), i)].at(-1);
      if (before !== undefined && (NICK_RUNE.test(before) || before === "@" || before === "/" || before === ".")) continue;
      const hit = names.find(([w]) => {
        if (!low.startsWith(w, i + 1)) return false;
        const after = [...line.slice(i + 1 + w.length, i + 3 + w.length)][0];
        return after === undefined || !NICK_RUNE.test(after);
      });
      if (!hit) continue;
      out.push({ from: start + i, to: start + i + 1 + hit[0].length, nick: hit[1] });
      i += hit[0].length;
    }
  }
  return out;
}

/** Words cut at the nicks they call: plain runs and nicks, in order. */
export function nickParts(text: string, nicks: string[]): (string | { nick: string })[] {
  const out: (string | { nick: string })[] = [];
  let at = 0;
  for (const s of nickSpans(text, nicks)) {
    if (s.from > at) out.push(text.slice(at, s.from));
    out.push({ nick: s.nick });
    at = s.to;
  }
  if (at < text.length) out.push(text.slice(at));
  return out;
}

/** The address a nick's link in Markdown has (chat/markdown.tsx draws it: NickLink). */
export const nickURL = (nick: string) => `ir:member/${encodeURIComponent(nick)}`;
export const nickOfURL = (url: unknown) => {
  const m = typeof url === "string" ? /^ir:member\/([^/\s]+)$/.exec(url) : null;
  try {
    return m ? decodeURIComponent(m[1]!) : undefined;
  } catch {
    return undefined;
  }
};

/** An agent's Markdown with each nick it calls a link, drawn as the nick's mark. */
export function linkNicks(text: string, nicks: string[]): string {
  let out = "";
  let at = 0;
  for (const s of nickSpans(text, nicks)) {
    out += `${text.slice(at, s.from)}[@${s.nick}](${nickURL(s.nick)})`;
    at = s.to;
  }
  return out + text.slice(at);
}

/** words put into text over [a, b) (the caret, a selection, an @word being typed): one space
 *  before them unless at a line's start or after one, one after them (a space already there is
 *  that one); the caret after it. The one way a mention goes into the composer: picked from the
 *  @ list, a card's 提及, a quote's @, a session or a place named. */
export function insertWords(text: string, a: number, b: number, words: string): { text: string; caret: number } {
  const head = text.slice(0, a);
  const before = head + (head && !/\s$/.test(head) ? " " : "");
  return { text: `${before}${words} ${text.slice(b).replace(/^ /, "")}`, caret: before.length + words.length + 1 };
}

/** "@nick " put into text over [a, b), as insertWords. */
export const insertNick = (text: string, a: number, b: number, nick: string) => insertWords(text, a, b, `@${nick}`);

/** Whether text begins with @name, as a name standing by itself (case does not matter). */
export const startsAt = (text: string, name: string) => nickSpans(text.trimStart(), [name]).some((s) => s.from === 0);

/** The name a quote of m puts at the composer's start (kumo, 2026-11-21: as QQ does): an agent's
 *  nick, a person's or an app's name; none for one's own words (mine), a script's, Iridium's. */
export function quoteAt(m: Pick<Msg, "author">, mine: boolean): string | undefined {
  const a = m.author;
  if (a.kind === "agent") return a.nick || undefined;
  if (a.kind === "person" && !mine) return a.name || undefined;
  return undefined;
}

/** A member as the @ list and the words that name it show it: an agent; or (person) a login or
 *  an app, its name as nick, its app (or 应用) as name, its avatar. */
export type AgentRow = { session: string; member: string; agent: Agent; nick: string; name: string; status: Status; on: Trigger[]; avatar?: string; person?: boolean };

/** Whether user (a Satori user id) may be talked to alone (docs/channels.md "私聊"): the host, or
 *  one of logins; not an app, nor an author that is no login (the server has no one to open the
 *  channel with). */
export const mayDM = (user: string, logins: { id: string }[]) => user === "local" || logins.some((l) => l.id === user);

/** The Satori user id of a person's member string: a login's own id ("<app>/<login>"), an app's
 *  for all of its logins ("<app>/*"). */
export function userOf(member: string): string {
  const i = member.indexOf("/");
  const login = member.slice(i + 1);
  return i < 0 || login !== "*" ? login : member.slice(0, i);
}

/** Whether a member is the channel's own agent: the session the channel is. It speaks there
 *  straight out, and is not taken out of it. */
export const ownAgent = (channel: string, m: { session?: string | null }) => !!m.session && m.session === channel;

/** The agents an @q may call: nicks that start with it first, then nicks and names that have it. */
export function matchAgents<T extends { nick: string; name: string }>(rows: T[], q: string): T[] {
  const w = q.toLowerCase();
  const rank = (r: T) => (!w || r.nick.toLowerCase().startsWith(w) ? 0 : r.nick.toLowerCase().includes(w) ? 1 : r.name.toLowerCase().includes(w) ? 2 : 3);
  return rows
    .map((r) => [rank(r), r] as const)
    .filter(([n]) => n < 3)
    .sort((a, b) => a[0] - b[0])
    .map(([, r]) => r);
}

/** Whether a session is a channel of several agents. */
export const isMulti = (s?: Pick<Session, "agents">) => (s?.agents?.length ?? 0) > 1;

/** One ask in a channel's queue (docs/channels.md "提问"): whose, and when the page first saw it. */
export type Queued<T> = { session: string; ask: T; at: number };

/** The asks of a channel's agents as one queue, oldest first (kumo, 2026-11-15). An ask has no
 *  time of its own: the first time the page sees it (seen, by card, kept across draws; those gone
 *  are dropped from it), in the agents' order for those seen at once. */
export function askQueue<T extends { requestId: string; card?: { id: string } }>(per: { session: string; asks: T[] }[], seen: Map<string, number>, now = Date.now()): Queued<T>[] {
  const out: Queued<T>[] = [];
  const here = new Set<string>();
  for (const p of per)
    for (const a of p.asks) {
      const key = a.card?.id ?? `ask:${p.session}:${a.requestId}`;
      here.add(key);
      if (!seen.has(key)) seen.set(key, now);
      out.push({ session: p.session, ask: a, at: seen.get(key)! });
    }
  for (const k of [...seen.keys()]) if (!here.has(k)) seen.delete(k);
  return out.sort((a, b) => a.at - b.at);
}

const WORKING = new Set<Status>(["running", "compacting", "starting"]);

/** A channel's lamp, from its agents': amber while one waits on you, else green while one works,
 *  else its own session's. */
export function channelLamp(own: Status, members: Status[]): Status {
  if (members.includes("waiting")) return "waiting";
  if (WORKING.has(own)) return own;
  return members.some((s) => WORKING.has(s)) ? "running" : own;
}

/** The states the stream is asked for besides the channel's own: its other agents' sessions
 *  first, then their contexts, as far as it has room (stream.ts keysFor). */
export const watchExtra = (channel: string, sessions: string[]) => {
  const rest = sessions.filter((s) => s !== channel);
  return [...rest.map((s) => `session:${s}`), ...rest.map((s) => `context:${s}`)];
};

/** What an agent was given of a channel, as its own record has it (internal/ctx/assemble
 *  ChannelTurn: "#n: words", then a <channel-context> block): the message's number and its
 *  words alone; channel: the channel it was called in, when that is not its own (it is in it by a
 *  subscription, docs/channels.md). null: not such a message. */
export type GivenTurn = { seq: number; words: string; context: string; channel?: { id: string; name: string } };

export function givenTurn(text: string): GivenTurn | null {
  // A reaction given is one too: "#n [who reacted 👍 to: …]" (docs/channels.md "表情").
  const m = /^#(\d+)(?:: | (?=\[))([\s\S]*?)\n\n<channel-context>\n([\s\S]*)$/.exec(text);
  if (!m) return null;
  const c = /^You are \S+, called in the channel "([\s\S]*?)"(?: \(([0-9a-zA-Z_-]+)\))?, which people/.exec(m[3]!);
  // What Iridium added: the block's lines, and what follows it (how to read more).
  const context = m[3]!.replace(/\n?<\/channel-context>/, "").trim();
  return { seq: Number(m[1]), words: m[2]!, context, ...(c ? { channel: { id: c[2] ?? "", name: c[1]! } } : {}) };
}

/** A part of what an agent was given (Injection.record): where it is from, the values its slots
 *  took, the messages it cites (#n). */
export type GivenPart = { source: string; values: [string, string][]; refs: string[] };

/** What an injection was (its record's kind): the system prompt, a channel's message, another
 *  line given (a message, Iridium's own), what ir was answered with. */
export type GivenKind = "system" | "turn" | "message" | "ir";
export function givenKind(record: unknown): GivenKind {
  const k = (record as { kind?: unknown } | null)?.kind;
  return k === "system" ? "system" : k === "turn-prefix" ? "turn" : k === "tool-result" ? "ir" : "message";
}
/** The whole text it was given (records kept before it was kept: none). */
export const givenText = (record: unknown): string => {
  const x = (record as { text?: unknown } | null)?.text;
  return typeof x === "string" ? x : "";
};

export function givenParts(record: unknown): GivenPart[] {
  const parts = (record as { parts?: unknown } | null)?.parts;
  if (!Array.isArray(parts)) return [];
  return parts.map((p: { source?: unknown; values?: unknown; refs?: unknown }) => ({
    source: typeof p.source === "string" ? p.source : "",
    values: p.values && typeof p.values === "object" ? Object.entries(p.values as Record<string, unknown>).map(([k, v]) => [k, String(v)] as [string, string]) : [],
    refs: Array.isArray(p.refs) ? p.refs.flatMap((r: { kind?: unknown; id?: unknown }) => (r?.kind === "message" && r.id !== undefined ? [`#${String(r.id)}`] : [])) : [],
  }));
}

/** A human-readable composition item for an injection record, grouped and filtered from raw
 *  template part ids. */
export type DescribedPart = {
  label: string;
  detail?: string;
  refs?: string[];
  values?: [string, string][];
  raw: string;
};

const GLUE_PARTS = new Set([
  "prompts:nl",
  "prompts:channel.context.open",
  "prompts:channel.context.close",
  "prompts:channel.context.line-end",
  "prompts:channel.context.line-did",
  "prompts:channel.quote-end",
  "prompts:channel.reaction-end",
  "prompts:activity.sep",
]);

const CLI_PARTS = new Set(["prompts:host.intro", "prompts:remote.intro", "prompts:remote.intro-bare", "irhelp:agent", "irhelp:remote", "irhelp:layers"]);
const ACTIVITY_PREFIX = "prompts:activity.";

const valOf = (p: GivenPart, key: string): string => p.values.find(([k]) => k === key)?.[1] ?? "";
const cleanFrom = (s: string): string => s.replace(/^[,\s，]+/, "").trim();

const PART_LABELS: Record<string, [string, string]> = {
  "prompts:host.secrets-files": ["密钥与文件规则", "Secrets & file links"],
  "prompts:host.manual": ["手动审批说明", "Manual approval guide"],
  "prompts:where.host": ["运行环境", "Environment"],
  "prompts:where.remote": ["远程运行环境", "Remote environment"],
  "prompts:names": ["链接格式", "Link syntax"],
  "prompts:title": ["会话命名规则", "Session title rule"],
  "prompts:channel.system": ["频道会话说明", "Channel session guide"],
  "prompts:lang": ["语言偏好", "Language"],
  "prompts:self.who": ["当前身份", "Identity"],
  "prompts:self.tools": ["记忆检索用法", "Recall usage"],
  "prompts:self.remember": ["记忆写入规则", "Memory rule"],
  "prompts:self.profile": ["资料修改用法", "Profile usage"],
  "prompts:self.helpers": ["子代理与协作规则", "Helpers & subagents"],
  "prompts:channel.context.rename": ["改名提示", "Rename prompt"],
  "prompts:channel.context.again": ["频道切换提示", "Channel reminder"],
  "prompts:channel.context.again-id": ["频道切换提示", "Channel reminder"],
  "prompts:channel.context.skipped": ["已省略早期消息", "Skipped messages"],
  "prompts:channel.context.recalled": ["已撤回消息", "Recalled messages"],
  "prompts:channel.context.pull": ["拉取历史用法", "History lookup usage"],
  "prompts:channel.context.react": ["表情回应用法", "Reaction usage"],
  "prompts:channel.send.unseen": ["发送后未读补发", "Unseen after send"],
  "prompts:channel.send.behind": ["发送被拒（落后过多）", "Send rejected (behind)"],
  "prompts:channel.send.named": ["发送被拒（有未读提及）", "Send rejected (mentioned)"],
  "prompts:channel.silent": ["成员未发话通知", "Silent agent notice"],
  "prompts:message.tag": ["发送者标记", "Sender tag"],
  "prompts:message.attachment": ["附件", "Attachment"],
  "prompts:ask.denied": ["审批拒绝", "Approval denied"],
  input: ["正文", "Content"],
};

/** Turns raw injection record parts into readable composition rows: syntactic glue is hidden,
 *  multi-part sections (CLI guide, memory, unread messages, etc.) are grouped, and internal
 *  template ids are kept only in `raw` for hover inspection. */
export function describeGivenParts(record: unknown, lang: "zh" | "en" = "zh"): DescribedPart[] {
  const parts = givenParts(record);
  const zh = lang === "zh";
  const pick = (pair: [string, string]) => (zh ? pair[0] : pair[1]);
  const out: DescribedPart[] = [];
  for (let i = 0; i < parts.length; ) {
    const p = parts[i]!;
    if (GLUE_PARTS.has(p.source)) {
      i++;
      continue;
    }
    // Group Iridium CLI intro + help layers.
    if (CLI_PARTS.has(p.source)) {
      const raws: string[] = [];
      let remote = p.source.startsWith("prompts:remote.");
      while (i < parts.length && CLI_PARTS.has(parts[i]!.source)) {
        raws.push(parts[i]!.source);
        if (parts[i]!.source.startsWith("prompts:remote.")) remote = true;
        i++;
      }
      out.push({
        label: remote ? pick(["远程 Iridium 说明", "Remote Iridium guide"]) : pick(["Iridium CLI 说明", "Iridium CLI guide"]),
        raw: raws.join(", "),
      });
      continue;
    }
    // Group memory section.
    if (p.source === "prompts:self.memory") {
      const raws = [p.source];
      const vals: [string, string][] = [];
      let more = "";
      i++;
      while (i < parts.length) {
        const s = parts[i]!.source;
        if (s === "prompts:self.memory-line") {
          raws.push(s);
          const date = valOf(parts[i]!, "date");
          const from = cleanFrom(valOf(parts[i]!, "from"));
          if (date || from) vals.push([date || "—", from]);
          i++;
        } else if (s === "prompts:self.memory-more") {
          raws.push(s);
          more = valOf(parts[i]!, "more");
          i++;
        } else if (s === "input" || s === "prompts:nl") {
          i++;
        } else {
          break;
        }
      }
      const n = vals.length;
      const count = zh ? `${n} 条${more ? `（另有 ${more} 条）` : ""}` : `${n}${more ? ` (+${more} more)` : ""}`;
      out.push({
        label: pick(["记忆", "Memory"]),
        ...(n > 0 ? { detail: count, values: vals } : {}),
        raw: [...new Set(raws)].join(", "),
      });
      continue;
    }
    // Group channels list section.
    if (p.source === "prompts:self.channels") {
      const raws = [p.source];
      let n = 0;
      i++;
      while (i < parts.length) {
        const s = parts[i]!.source;
        if (s === "prompts:self.channel-line" || s === "prompts:self.channel-quiet") {
          raws.push(s);
          n++;
          i++;
        } else if (s === "input" || s === "prompts:nl") {
          i++;
        } else {
          break;
        }
      }
      out.push({
        label: pick(["所在频道", "Channels"]),
        ...(n > 0 ? { detail: zh ? `${n} 个` : `${n}` } : {}),
        raw: [...new Set(raws)].join(", "),
      });
      continue;
    }
    // Group earlier contexts section.
    if (p.source === "prompts:self.before") {
      const raws = [p.source];
      let n = 0;
      i++;
      while (i < parts.length) {
        const s = parts[i]!.source;
        if (s === "prompts:self.before-line") {
          raws.push(s);
          n++;
          i++;
        } else if (s === "input" || s === "prompts:nl") {
          i++;
        } else {
          break;
        }
      }
      out.push({
        label: pick(["清空前上下文", "Earlier contexts"]),
        ...(n > 0 ? { detail: zh ? `${n} 段` : `${n}` } : {}),
        raw: [...new Set(raws)].join(", "),
      });
      continue;
    }
    // Group trigger message (+ optional quote + body input).
    if (p.source === "prompts:channel.message") {
      const raws = [p.source];
      const seq = valOf(p, "seq");
      const refs = seq ? [`#${seq}`] : [...p.refs];
      const vals: [string, string][] = [];
      i++;
      if (i < parts.length && parts[i]!.source === "prompts:channel.quote") {
        const qp = parts[i]!;
        raws.push(qp.source);
        const qseq = valOf(qp, "seq");
        const qwho = valOf(qp, "who");
        vals.push([pick(["引用", "Quote"]), `#${qseq}${qwho ? ` ${qwho}` : ""}`]);
        i++;
        while (i < parts.length && (parts[i]!.source === "input" || parts[i]!.source === "prompts:channel.quote-end")) i++;
      }
      if (i < parts.length && parts[i]!.source === "input") {
        for (const r of parts[i]!.refs) if (!refs.includes(r)) refs.push(r);
        i++;
      }
      out.push({
        label: pick(["触发消息", "Trigger message"]),
        ...(refs.length ? { refs } : {}),
        ...(vals.length ? { values: vals } : {}),
        raw: raws.join(", "),
      });
      continue;
    }
    // Group trigger reaction.
    if (p.source === "prompts:channel.reaction") {
      const raws = [p.source];
      const seq = valOf(p, "seq");
      const who = valOf(p, "who");
      const emoji = valOf(p, "emoji");
      i++;
      while (i < parts.length && (parts[i]!.source === "input" || parts[i]!.source === "prompts:channel.reaction-end")) i++;
      out.push({
        label: pick(["表情回应", "Reaction"]),
        ...(who || emoji ? { detail: [who, emoji].filter(Boolean).join(" ") } : {}),
        ...(seq ? { refs: [`#${seq}`] } : {}),
        raw: raws.join(", "),
      });
      continue;
    }
    // Group channel standing rules & members.
    if (p.source === "prompts:channel.context.guest" || p.source === "prompts:channel.context.guest-bare" || p.source === "prompts:channel.context.members") {
      const raws = [p.source];
      const vals: [string, string][] = [];
      const name = valOf(p, "name");
      const channel = valOf(p, "channel");
      const you = valOf(p, "you");
      if (name || channel) vals.push([pick(["频道", "Channel"]), name && channel && name !== channel ? `${name} (${channel})` : name || channel]);
      if (you) vals.push([pick(["称呼", "Nick"]), you]);
      i++;
      const sub = new Set(["prompts:channel.context.place", "prompts:channel.context.away", "prompts:channel.context.scripts", "prompts:channel.context.recv"]);
      while (i < parts.length && sub.has(parts[i]!.source)) {
        raws.push(parts[i]!.source);
        i++;
      }
      out.push({
        label: pick(["频道规则与成员", "Channel rules & members"]),
        ...(vals.length ? { values: vals } : {}),
        raw: raws.join(", "),
      });
      continue;
    }
    // Group pull + react helper block.
    if (p.source === "prompts:channel.context.pull" && parts[i + 1]?.source === "prompts:channel.context.react") {
      out.push({
        label: pick(["历史与表情用法", "History & reaction usage"]),
        raw: "prompts:channel.context.pull, prompts:channel.context.react",
      });
      i += 2;
      continue;
    }
    // Group full unread messages.
    if (p.source === "prompts:channel.context.full") {
      const raws = [p.source];
      const refs: string[] = [];
      i++;
      while (i < parts.length) {
        const s = parts[i]!.source;
        if (s === "prompts:channel.context.line") {
          raws.push(s);
          const seq = valOf(parts[i]!, "seq");
          if (seq && !refs.includes(`#${seq}`)) refs.push(`#${seq}`);
          i++;
        } else if (
          s === "prompts:channel.context.line-did" ||
          s === "prompts:channel.context.line-end" ||
          s === "prompts:channel.context.cut" ||
          s === "input" ||
          s === "prompts:nl" ||
          s.startsWith(ACTIVITY_PREFIX)
        ) {
          i++;
        } else {
          break;
        }
      }
      out.push({
        label: pick(["未读消息", "Unread messages"]),
        ...(refs.length ? { refs } : {}),
        raw: [...new Set(raws)].join(", "),
      });
      continue;
    }
    // Group summarized unread turns.
    if (p.source === "prompts:channel.context.listed") {
      const raws = [p.source];
      let n = 0;
      i++;
      while (i < parts.length) {
        const s = parts[i]!.source;
        if (s === "prompts:channel.context.turn") {
          raws.push(s);
          n++;
          i++;
        } else if (s === "prompts:channel.context.answered" || s === "prompts:nl") {
          i++;
        } else {
          break;
        }
      }
      out.push({
        label: pick(["未读摘要", "Unread summary"]),
        ...(n > 0 ? { detail: zh ? `${n} 轮` : `${n} turns` } : {}),
        raw: [...new Set(raws)].join(", "),
      });
      continue;
    }
    // Group other agents' work.
    if (p.source === "prompts:channel.context.work") {
      const raws = [p.source];
      const whos: string[] = [];
      i++;
      while (i < parts.length) {
        const s = parts[i]!.source;
        if (s === "prompts:channel.context.work-line") {
          raws.push(s);
          const who = valOf(parts[i]!, "who");
          if (who && !whos.includes(who)) whos.push(who);
          i++;
        } else if (s === "prompts:channel.context.work-files" || s === "prompts:nl" || s.startsWith(ACTIVITY_PREFIX)) {
          i++;
        } else {
          break;
        }
      }
      out.push({
        label: pick(["其他成员操作", "Other agents' work"]),
        ...(whos.length ? { detail: whos.join(", ") } : {}),
        raw: [...new Set(raws)].join(", "),
      });
      continue;
    }
    // Group profile changes.
    if (p.source === "prompts:channel.context.profiles") {
      const raws = [p.source];
      let n = 0;
      i++;
      while (i < parts.length) {
        const s = parts[i]!.source;
        if (s.startsWith("prompts:channel.context.profile-")) {
          raws.push(s);
          n++;
          i++;
        } else if (s === "input" || s === "prompts:nl") {
          i++;
        } else {
          break;
        }
      }
      out.push({
        label: pick(["资料变更", "Profile changes"]),
        ...(n > 0 ? { detail: zh ? `${n} 项` : `${n}` } : {}),
        raw: [...new Set(raws)].join(", "),
      });
      continue;
    }
    // Single-part items with concise inline details.
    const known = PART_LABELS[p.source];
    if (p.source === "prompts:where.host") {
      const os = valOf(p, "os");
      const folder = valOf(p, "folder");
      out.push({ label: pick(known!), detail: [os, folder].filter(Boolean).join(" · "), raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:where.remote") {
      const computer = valOf(p, "computer");
      const folder = valOf(p, "folder");
      out.push({ label: pick(known!), detail: [computer, folder].filter(Boolean).join(" · "), raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:lang") {
      out.push({ label: pick(known!), detail: valOf(p, "language"), raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:self.who") {
      out.push({ label: pick(known!), detail: valOf(p, "nick"), raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:channel.context.again" || p.source === "prompts:channel.context.again-id") {
      const name = valOf(p, "name");
      const channel = valOf(p, "channel");
      out.push({ label: pick(known!), detail: name && channel && name !== channel ? `${name} (${channel})` : name || channel, raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:channel.context.skipped") {
      const from = valOf(p, "from"), to = valOf(p, "to"), count = valOf(p, "count");
      out.push({ label: pick(known!), detail: from && to ? `#${from}–#${to}${count ? ` (${count})` : ""}` : count, raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:channel.context.recalled") {
      out.push({ label: pick(known!), detail: valOf(p, "seqs"), raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:message.tag") {
      out.push({ label: pick(known!), detail: valOf(p, "tag"), raw: p.source });
      i++;
      continue;
    }
    if (p.source === "prompts:message.attachment") {
      out.push({ label: pick(known!), detail: valOf(p, "path"), raw: p.source });
      i++;
      continue;
    }
    out.push({
      label: known ? pick(known) : p.source,
      ...(p.refs.length ? { refs: p.refs } : {}),
      ...(p.values.length ? { values: p.values } : {}),
      raw: p.source,
    });
    i++;
  }
  return out;
}
