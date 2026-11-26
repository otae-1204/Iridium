// The work key (kumo + Neon, 2026-11-26, in place of the floating work bar): in the channel's top
// bar, left of the members' key and drawn as it is, there only while one of its agents works: 回复中
// while a turn runs, 后台 while only its background tasks do (Claude's subagents and commands sent
// to the background, Codex's subagents, dsh's jobs). Folded it is one shape whatever the count: the
// faces of those at work overlapped (3 on a computer, 2 on a phone), how many they are, a tiny faint
// dot breathing; it grows only with the count. Pressed, the top bar's own panel: each agent a group,
// its row (回复中 or 后台 and how long; pressed, to the message it writes; ■ stops the turn), its
// tasks under it (sort, what, steps, how long; ■ where the agent stops one alone; a subagent's
// pressed, its record). The members' old 在干活的子代理 are its tasks too. A task ended stays a
// moment, marked done. A stop key asks once (确认停止) before it stops. In a chat of one agent the
// key is there only for its tasks: its turn has the message's own line.
import { Menu } from "@base-ui/react/menu";
import { Bot, ListChecks, Square, SquareTerminal } from "lucide-react";
import { useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { api } from "../../api/client";
import type { Session, SessionState } from "../../api/types.gen";
import { t } from "../../i18n";
import { openRecord } from "../../route";
import { useList } from "../../store";
import { working } from "../../tree";
import { errText, toast } from "../../toast";
import { cn, MenuPortal, PHONE, Tip, useMedia } from "../../ui";
import { stopTurn } from "../live";
import { subName } from "../subwork";
import { MemberFace } from "./face";
import { ITEM, POPUP } from "./invite";
import type { AgentRow, Msg } from "./lib";
import { clockOf, DONE_FOR, folded, looseSubs, phaseOf, settle, settling, subOf, taskKind, workers, writingOf, type Shown, type Worker } from "./workstate";

/** What the channel's page lends the key while its conversation is drawn: its agents and their
 *  states, its messages, its way to one of them; may: the one looking may stop; solo: one agent. */
export type WorkLent = { agents: AgentRow[]; stateAt: (session: string) => SessionState | undefined; msgs: Msg[]; jump: (n: number) => void; may: boolean; solo: boolean };
const lent = new Map<string, WorkLent>();
const heard = new Set<() => void>();
const tell = () => heard.forEach((f) => f());
/** The page lends this at each of its draws. */
export function lendWork(channel: string, l: WorkLent) {
  lent.set(channel, l);
  tell();
}
/** The page gone: the key with it. */
export function unlendWork(channel: string) {
  if (lent.delete(channel)) tell();
}
const hear = (f: () => void) => (heard.add(f), () => void heard.delete(f));
const useLent = (channel: string) => useSyncExternalStore(hear, () => lent.get(channel));

/** Now, again each second while on. */
function useNow(on: boolean): number {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    if (!on) return;
    setNow(Date.now());
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [on]);
  return now;
}

/** Those at work in the channel and their tasks, those just ended kept DONE_FOR. */
function useWorkers(id: string, l: WorkLent | undefined, list: Session[] | undefined) {
  const [, redraw] = useState(0);
  // The tasks listed, by session, those just ended with them; when each was first seen, for one
  // that does not say when it began, and when each turn was.
  const kept = useRef(new Map<string, Shown[]>());
  const seen = useRef(new Map<string, number>());
  const at = Date.now();
  const first = (k: string) => seen.current.get(k) ?? (seen.current.set(k, at), at);
  const agents = l?.agents ?? [];
  const stateAt = l?.stateAt ?? (() => undefined);
  const own = (s: string) => stateAt(s)?.tasks ?? [];
  // Its subagents at work no task stands for (not in a chat of one: only its background shows).
  const loose = l && !l.solo ? looseSubs(list, id, agents.map((a) => a.session), own, (s) => working(s, list?.find((x) => x.id === s.parent), at), subName) : new Map();
  const tasksOf = (a: AgentRow) => {
    const now = [...own(a.session), ...(loose.get(a.session) ?? [])];
    const ls = settle(kept.current.get(a.session) ?? [], now, at);
    kept.current.set(a.session, ls);
    return ls.map((x) => (x.since ? x : { ...x, since: first(`${a.session} ${x.id}`) }));
  };
  const ws = l ? workers(agents, (a) => phaseOf(stateAt(a.session)), tasksOf, l.solo) : [];
  // One just ended goes DONE_FOR after: drawn again then.
  const ending = ws.some((w) => settling(w.tasks));
  useEffect(() => {
    if (!ending) return;
    const tm = setTimeout(() => redraw((n) => n + 1), DONE_FOR / 4);
    return () => clearTimeout(tm);
  });
  const since = (w: Worker<AgentRow>) => {
    const m = l && writingOf(l.msgs, w.a.session);
    return m?.work?.at ?? m?.time ?? first(`turn ${w.a.session} ${stateAt(w.a.session)?.proc ?? 0}`);
  };
  return { ws, since };
}

/** The key, left of the members' key, and its panel. */
export function WorkKey({ id }: { id: string }) {
  const l = useLent(id);
  const list = useList();
  const phone = useMedia(PHONE);
  const { ws, since } = useWorkers(id, l, list);
  const [open, setOpen] = useState(false);
  const on = ws.length > 0;
  if (open && !on) setOpen(false);
  const now = useNow(open);
  if (!l || !on) return null;
  const { shown: faces } = folded(ws, phone ? 2 : 3);
  const label = t("bar.label");
  return (
    <Menu.Root open={open} onOpenChange={setOpen}>
      <Tip label={label} side="bottom">
        <Menu.Trigger
          render={
            <button type="button" aria-label={`${label} ${ws.length}`} className="g-fade g-press flex h-8 items-center gap-1 rounded-md px-2 text-ink-2 hover:bg-fill hover:text-ink data-popup-open:bg-fill pointer-coarse:h-10">
              <span className="flex items-center">
                {faces.map((w, i) => (
                  <span key={w.a.session} className="grid rounded-full ring-2 ring-paper" style={{ marginLeft: i ? -5 : 0, zIndex: 3 - i }}>
                    <MemberFace member={w.a.member} agent={w.a.agent} size={18} />
                  </span>
                ))}
              </span>
              <span className="text-xs tabular-nums">{ws.length}</span>
              <span aria-hidden className="wk-dot" />
            </button>
          }
        />
      </Tip>
      <MenuPortal>
        <Menu.Positioner side="bottom" align="end" sideOffset={6} collisionPadding={8} className="z-50">
          <Menu.Popup finalFocus={false} className={cn(POPUP, "no-bar flex max-h-[min(480px,50dvh,var(--available-height))] flex-col overflow-y-auto p-1.5", phone ? "w-[calc(100vw-16px)]" : "w-[min(400px,calc(100vw-16px))]")}>
            <div className="g-stagger flex flex-col">
              {ws.map((w) => (
                <Group
                  key={w.a.session}
                  w={w}
                  now={now}
                  since={since(w)}
                  may={l.may}
                  jump={() => {
                    const m = writingOf(l.msgs, w.a.session) ?? l.msgs.findLast((x) => x.author.kind === "agent" && x.author.session === w.a.session);
                    if (m) l.jump(m.n);
                  }}
                  record={(task) => {
                    const s = subOf(list, w.a.session, task);
                    return s ? () => openRecord(s.id) : undefined;
                  }}
                />
              ))}
            </div>
          </Menu.Popup>
        </Menu.Positioner>
      </MenuPortal>
    </Menu.Root>
  );
}

/** A stop key that asks once: pressed, it turns red and says 确认停止; pressed again, it stops.
 *  Put away with the panel, or a while after, it asks again. */
function StopSure({ label, run }: { label: string; run: () => Promise<unknown> }) {
  const [armed, setArmed] = useState(false);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const id = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(id);
  }, [armed]);
  return (
    <button
      type="button"
      aria-label={armed ? t("bar.sure") : label}
      disabled={busy}
      onClick={(e) => {
        e.stopPropagation();
        if (!armed) return setArmed(true);
        setArmed(false);
        setBusy(true);
        void run().finally(() => setBusy(false));
      }}
      onBlur={() => setArmed(false)}
      className={cn("g-press grid h-6 shrink-0 place-items-center rounded-md text-xs outline-none disabled:opacity-40 pointer-coarse:h-8", armed ? "bg-fill px-2 text-err" : "w-6 text-ink-3 hover:bg-fill hover:text-ink pointer-coarse:w-8")}
    >
      {armed ? t("bar.sure") : <Square size={10} fill="currentColor" />}
    </button>
  );
}

/** The key's room where a row has none, so the timers stand in one column. */
const NoKey = () => <span aria-hidden className="w-6 shrink-0 pointer-coarse:w-8" />;

const TIMER = "w-[7ch] shrink-0 text-right text-xs text-ink-4 tabular-nums";
const ROW = "flex min-w-0 items-center gap-1";

const kindIcon = { agent: Bot, shell: SquareTerminal, task: ListChecks } as const;
const kindWord = { agent: "bar.agent", shell: "bar.shell", task: "bar.task" } as const;

/** One agent's group in the panel: its row, then its tasks. */
function Group({ w, now, since, may, jump, record }: { w: Worker<AgentRow>; now: number; since: number; may: boolean; jump: () => void; record: (t: Shown) => (() => void) | undefined }) {
  const replying = w.phase === "reply";
  return (
    <>
      <div className={ROW}>
        <Menu.Item onClick={jump} className={cn(ITEM, "min-w-0 flex-1 gap-2")}>
          <MemberFace member={w.a.member} agent={w.a.agent} size={18} />
          <span className="min-w-0 truncate font-medium text-ink">{w.a.nick || w.a.name}</span>
          <span className="shrink-0 text-sm text-ink-3">{replying ? t("bar.reply") : t("bar.back", { n: w.running })}</span>
          <span className="flex-1" />
          {replying && <span className={TIMER}>{clockOf(now - since)}</span>}
        </Menu.Item>
        {replying && may ? <StopSure label={t("bar.stop")} run={() => stopTurn(w.a.session)} /> : <NoKey />}
      </div>
      {w.tasks.map((x) => (
        <TaskRow key={x.id} session={w.a.session} task={x} now={now} may={may} open={record(x)} />
      ))}
    </>
  );
}

/** A task's row, under its agent's: its sort, what it does, its steps, how long; a subagent's
 *  with a session of its own opens its record. */
function TaskRow({ session, task, now, may, open }: { session: string; task: Shown; now: number; may: boolean; open?: () => void }) {
  const kind = taskKind(task);
  const Icon = kindIcon[kind];
  const done = task.ended !== undefined;
  const body: ReactNode = (
    <>
      <Icon size={14} aria-hidden className="shrink-0 text-ink-3" />
      <span className="shrink-0 text-sm text-ink-2">{t(kindWord[kind])}</span>
      <span className="min-w-[3rem] flex-auto truncate text-sm text-ink-3">{task.description}</span>
      {/* Its steps where the agent says, what it is at after them, given up first where the row is narrow. */}
      {!done && !!task.steps && <span className="shrink-0 text-xs text-ink-4 tabular-nums">{t("bar.steps", { n: task.steps })}</span>}
      {!done && !!task.now && <span className="min-w-0 shrink-[4] truncate text-xs text-ink-4">{task.now}</span>}
      <span className={cn(TIMER, done && "text-ink-3")}>{done ? t("bar.done") : clockOf(now - (task.since ?? now))}</span>
    </>
  );
  return (
    <div data-done={done || undefined} className={cn(ROW, "wb-task")}>
      <Menu.Item disabled={!open || done} onClick={open} className={cn(ITEM, "min-w-0 flex-1 gap-2 pl-8 data-disabled:data-highlighted:bg-transparent")}>
        {body}
      </Menu.Item>
      {task.stop && may && !done ? (
        <StopSure
          label={t("bar.stop")}
          run={() =>
            api.stopTask(session, task.id).then(
              () => {},
              (e) => void toast(t("err.stop"), { error: true, description: errText(e) }),
            )
          }
        />
      ) : (
        <NoKey />
      )}
    </div>
  );
}
