// The work bar (kumo, 2026-11-26): at the channel's top right, floating under its top bar, who of
// its agents is at work and how: 回复中 while a turn runs, 后台 while only its background tasks do
// (Claude's subagents and commands sent to the background, Codex's subagents, dsh's jobs). Gone
// when no one works. Its shell is the composer's (SHELL, one line high). Folded: one agent, its face,
// nick and state; more, their faces side by side (three, then +N), each with its state: a turning
// arc for a turn, a slow breathing ring for background work alone, the number of its tasks at its
// lower right. A face pressed opens one panel under the bar with that agent's group open, the
// others folded to a line; +N or the one agent's bar, all of them open. A group: its row (its turn
// and how long; pressed, to the message it writes; ■ stops the turn), its tasks under it (sort,
// what, steps, how long, ■ where the agent stops one alone; a subagent's pressed, its record). A
// task ended stays a moment, marked done. A stop key asks once (确认停止) before it stops.
// In a chat of one agent the bar is there only for its tasks: its turn has the message's own line.
import { Popover } from "@base-ui/react/popover";
import { Bot, ChevronRight, ListChecks, Square, SquareTerminal } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { api } from "../../api/client";
import type { SessionState, Session } from "../../api/types.gen";
import { t } from "../../i18n";
import { useList } from "../../store";
import { errText, toast } from "../../toast";
import { cn, PHONE, PopSurface, SHELL, SHELL_LINE, useMedia } from "../../ui";
import { stopTurn } from "../live";
import { MemberFace } from "./face";
import type { AgentRow, Msg } from "./lib";
import { clockOf, DONE_FOR, folded, phaseOf, settle, settling, taskKind, workers, writingOf, type Phase, type Shown, type Worker } from "./workstate";

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

/** A face with its state: the arc turning round it in a turn, the ring breathing for background
 *  work alone; n, its tasks running, at its lower right. */
export function StateFace({ a, phase, n, size }: { a: AgentRow; phase: Phase; n: number; size: number }) {
  return (
    <span className="wb-face" data-p={phase} style={{ width: size, height: size }}>
      <MemberFace member={a.member} agent={a.agent} size={size} />
      {n > 0 && <span className="wb-badge">{n}</span>}
    </span>
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
      className={cn("g-press grid h-6 shrink-0 place-items-center rounded-full text-xs outline-none disabled:opacity-40 pointer-coarse:h-8", armed ? "bg-fill px-2 text-err" : "w-6 text-ink-3 hover:bg-fill hover:text-ink pointer-coarse:w-8")}
    >
      {armed ? t("bar.sure") : <Square size={10} fill="currentColor" />}
    </button>
  );
}

/** The key's room where a row has none, so the timers stand in one column. */
const NoKey = () => <span aria-hidden className="w-6 shrink-0 pointer-coarse:w-8" />;

const TIMER = "w-[7ch] shrink-0 text-right text-xs text-ink-4 tabular-nums";
const ROW = "flex h-9 min-w-0 items-center gap-2 rounded-[12px] px-2 text-sm pointer-coarse:h-11";

const kindIcon = { agent: Bot, shell: SquareTerminal, task: ListChecks } as const;
const kindWord = { agent: "bar.agent", shell: "bar.shell", task: "bar.task" } as const;

/** A subagent's own session, where the list has it: Claude's by the call that started it, Codex's
 *  and dsh's by its id. */
const subOf = (list: Session[] | undefined, parent: string, task: Shown): Session | undefined =>
  list?.find((s) => s.subagent && ((!!task.call && s.parent === parent && s.call === task.call) || s.id === task.id));

/** The bar and its panel. stateAt: an agent's carried state; msgs: the channel's messages; jump:
 *  to message n; record: opens a session's read-only record; may: the one looking may stop. */
export function WorkBar({ agents, stateAt, msgs, jump, record, may, solo }: { agents: AgentRow[]; stateAt: (session: string) => SessionState | undefined; msgs: Msg[]; jump: (n: number) => void; record: (session: string) => void; may: boolean; solo: boolean }) {
  const list = useList();
  const phone = useMedia(PHONE);
  const [, redraw] = useState(0);
  // The tasks listed, by session, those just ended with them; when each was first seen, for one
  // that does not say when it began, and when each turn was.
  const kept = useRef(new Map<string, Shown[]>());
  const seen = useRef(new Map<string, number>());
  const at = Date.now();
  const first = (k: string) => seen.current.get(k) ?? (seen.current.set(k, at), at);
  const tasksOf = (a: AgentRow) => {
    const now = stateAt(a.session)?.tasks ?? [];
    const l = settle(kept.current.get(a.session) ?? [], now, at);
    kept.current.set(a.session, l);
    return l.map((x) => (x.since ? x : { ...x, since: first(`${a.session} ${x.id}`) }));
  };
  const ws = workers(agents, (a) => phaseOf(stateAt(a.session)), tasksOf, solo);
  // One just ended goes DONE_FOR after: drawn again then.
  const ending = ws.some((w) => settling(w.tasks));
  useEffect(() => {
    if (!ending) return;
    const id = setTimeout(() => redraw((n) => n + 1), DONE_FOR / 4);
    return () => clearTimeout(id);
  });
  const on = ws.length > 0;
  // What it showed last, kept while it fades out.
  const last = useRef<Worker<AgentRow>[]>([]);
  if (on) last.current = ws;
  const shown = on ? ws : last.current;
  const [open, setOpen] = useState(false);
  // The agent whose group is open alone; null: all of them.
  const [focus, setFocus] = useState<string | null>(null);
  if (open && !on) setOpen(false);
  const now = useNow(on);
  const bar = useRef<HTMLDivElement>(null);
  const press = (who: string | null) => {
    if (open && focus === who) return setOpen(false);
    setFocus(who);
    setOpen(true);
  };
  const { shown: faces, more } = folded(shown);
  const one = shown.length === 1 ? shown[0] : undefined;
  const turnSince = (w: Worker<AgentRow>) => {
    const m = writingOf(msgs, w.a.session);
    return m?.work?.at ?? m?.time ?? first(`turn ${w.a.session} ${stateAt(w.a.session)?.proc ?? 0}`);
  };
  if (!shown.length) return null;
  // On a phone the panel is as wide as the window, from where the bar is.
  const anchor = phone && bar.current ? { getBoundingClientRect: () => new DOMRect(8, bar.current!.getBoundingClientRect().top, window.innerWidth - 16, 0) } : bar.current;
  return (
    <div className="pointer-events-none absolute inset-x-0 top-14 z-[15] px-2.5 sm:px-5 pointer-coarse:top-15">
      <div className="mx-auto flex max-w-(--chat-w) justify-end">
        <div ref={bar} role="status" aria-label={t("bar.label")} data-on={on || undefined} inert={!on || undefined} className={cn(SHELL, SHELL_LINE, "wb-bar pointer-events-auto flex min-w-0 items-center gap-1 px-2.5")}>
          {one ? (
            <button type="button" aria-expanded={open} onClick={() => press(null)} className="g-press flex h-full min-w-0 items-center gap-2.5 rounded-full px-0.5 outline-none">
              <StateFace a={one.a} phase={one.phase} n={one.running} size={26} />
              <span className="min-w-0 truncate text-sm font-medium text-ink">{one.a.nick || one.a.name}</span>
              <span className="shrink-0 pr-1 text-sm text-ink-3">{one.phase === "reply" ? t("bar.reply") : t("bar.back", { n: one.running })}</span>
            </button>
          ) : (
            <>
              {faces.map((w) => (
                <button key={w.a.session} type="button" aria-label={`${w.a.nick || w.a.name} ${w.phase === "reply" ? t("bar.reply") : t("bar.back", { n: w.running })}`} aria-expanded={open && focus === w.a.session} onClick={() => press(w.a.session)} className="g-press grid size-9 shrink-0 place-items-center rounded-full outline-none">
                  <StateFace a={w.a} phase={w.phase} n={w.running} size={26} />
                </button>
              ))}
              {more > 0 && (
                <button type="button" aria-expanded={open && focus === null} onClick={() => press(null)} className="g-press grid h-9 min-w-9 shrink-0 place-items-center rounded-full px-1.5 text-sm text-ink-2 tabular-nums outline-none hover:text-ink">
                  +{more}
                </button>
              )}
            </>
          )}
        </div>
      </div>
      <Popover.Root
        open={open && on}
        onOpenChange={(o, d) => {
          // A press on the bar is the bar's own (another face, the same one to shut it).
          if (!o && d.event?.target instanceof Node && bar.current?.contains(d.event.target)) return;
          setOpen(o);
        }}
      >
        <PopSurface anchor={anchor} side="bottom" align={phone ? "start" : "end"} sideOffset={6} className={cn("flex max-h-[50dvh] flex-col gap-0.5 overflow-y-auto overscroll-contain rounded-[20px]! p-1.5", phone ? "w-[calc(100vw-16px)]" : "w-[min(420px,calc(100vw-16px))]")}>
          {shown.map((w) => (
            <Group
              key={w.a.session}
              w={w}
              now={now}
              since={turnSince(w)}
              unfolded={focus === null || focus === w.a.session}
              unfold={() => setFocus(null)}
              may={may}
              jump={() => {
                const m = writingOf(msgs, w.a.session) ?? msgs.findLast((x) => x.author.kind === "agent" && x.author.session === w.a.session);
                if (!m) return;
                setOpen(false);
                jump(m.n);
              }}
              open={(task) => {
                const s = subOf(list, w.a.session, task);
                return s ? () => (setOpen(false), record(s.id)) : undefined;
              }}
            />
          ))}
        </PopSurface>
      </Popover.Root>
    </div>
  );
}

/** One agent's group in the panel: its row, then its tasks; folded, its row alone, pressed to open. */
function Group({ w, now, since, unfolded, unfold, may, jump, open }: { w: Worker<AgentRow>; now: number; since: number; unfolded: boolean; unfold: () => void; may: boolean; jump: () => void; open: (t: Shown) => (() => void) | undefined }) {
  const replying = w.phase === "reply";
  return (
    <div className="flex min-w-0 flex-col">
      <div className={cn(ROW, "hover:bg-fill")}>
        <button type="button" aria-expanded={unfolded} onClick={unfolded ? jump : unfold} className="flex h-full min-w-0 flex-1 items-center gap-2 text-left outline-none">
          <StateFace a={w.a} phase={w.phase} n={0} size={20} />
          <span className="min-w-0 truncate font-medium text-ink">{w.a.nick || w.a.name}</span>
          <span className="shrink-0 text-ink-3">{replying ? t("bar.reply") : t("bar.back", { n: w.running })}</span>
          {!unfolded && <ChevronRight size={13} aria-hidden className="shrink-0 text-ink-4" />}
          <span className="flex-1" />
          {replying && <span className={TIMER}>{clockOf(now - since)}</span>}
        </button>
        {replying && may ? <StopSure label={t("bar.stop")} run={() => stopTurn(w.a.session)} /> : <NoKey />}
      </div>
      {unfolded && w.tasks.map((x) => <TaskRow key={x.id} session={w.a.session} task={x} now={now} may={may} open={open(x)} />)}
    </div>
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
      <span className="shrink-0 text-ink-2">{t(kindWord[kind])}</span>
      <span className="min-w-[3rem] flex-auto truncate text-ink-3">{task.description}</span>
      {/* Its steps where the agent says, what it is at after them, given up first where the row is narrow. */}
      {!done && !!task.steps && <span className="shrink-0 text-xs text-ink-4 tabular-nums">{t("bar.steps", { n: task.steps })}</span>}
      {!done && !!task.now && <span className="min-w-0 shrink-[4] truncate text-xs text-ink-4">{task.now}</span>}
      <span className={cn(TIMER, done && "text-ink-3")}>{done ? t("bar.done") : clockOf(now - (task.since ?? now))}</span>
    </>
  );
  return (
    <div data-done={done || undefined} className={cn(ROW, "wb-task pl-8", open && !done && "hover:bg-fill")}>
      {open && !done ? (
        <button type="button" onClick={open} className="flex h-full min-w-0 flex-1 items-center gap-2 text-left outline-none">
          {body}
        </button>
      ) : (
        <span className="flex h-full min-w-0 flex-1 items-center gap-2">{body}</span>
      )}
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
