// The work bar's state (workbar.tsx), pure for tests: who of a channel's agents is at work, and how.
// An agent is 回复中 while its turn runs, 后台 once its turn is over and tasks of its still run
// (Claude's background subagents and commands, Codex's subagents, dsh's jobs), else idle. A task
// just ended stays a moment, marked done, then goes.
import type { Task } from "../../api/types.gen";
import type { Msg } from "./lib";

export type Phase = "reply" | "back" | "idle";

/** Statuses of a turn going on: at work, compacting, or waiting on an answer in it. */
const TURN = new Set(["running", "compacting", "waiting"]);

/** An agent's phase by its session's status and tasks. */
export function phaseOf(st?: { status: string; tasks?: Task[] | null }): Phase {
  if (!st) return "idle";
  if (TURN.has(st.status)) return "reply";
  return st.tasks?.length ? "back" : "idle";
}

/** What a task is: a subagent, a shell command, or another sort. */
export function taskKind(t: Pick<Task, "type">): "agent" | "shell" | "task" {
  const k = (t.type ?? "").toLowerCase();
  if (k === "local_agent" || k === "remote_agent" || k.includes("subagent") || k === "agent") return "agent";
  if (k === "local_bash" || k.includes("shell") || k.includes("bash") || k === "exec" || k === "command" || k === "job") return "shell";
  return "task";
}

/** A task as the bar lists it: ended, when it was last seen running (it then stays DONE_FOR, marked done). */
export type Shown = Task & { ended?: number };

/** How long a task that ended stays in the bar, marked done. */
export const DONE_FOR = 4000;

/** The tasks to list now: those running, then those ended in the last DONE_FOR, in the order first
 *  seen. was: what was listed before (with ended ones); now: the agent's tasks now. */
export function settle(was: Shown[], now: Task[], at: number): Shown[] {
  const live = new Map(now.map((t) => [t.id, t]));
  const out: Shown[] = [];
  for (const t of was) {
    const n = live.get(t.id);
    if (n) out.push(n);
    else if (t.ended === undefined) out.push({ ...t, ended: at });
    else if (at - t.ended < DONE_FOR) out.push(t);
  }
  const seen = new Set(was.map((t) => t.id));
  for (const t of now) if (!seen.has(t.id)) out.push(t);
  return out;
}

/** Whether a list settled still has a task marked done that will go later. */
export const settling = (l: Shown[]) => l.some((t) => t.ended !== undefined);

/** One agent at work, as the bar draws it. */
export type Worker<A> = { a: A; phase: Phase; tasks: Shown[]; running: number };

/** The agents to show, in the channel's order: those at work (a turn, a task running or just
 *  ended). solo: a chat of one agent, shown only for its tasks (its turn has its own line). */
export function workers<A extends { session: string }>(agents: A[], phase: (a: A) => Phase, tasks: (a: A) => Shown[], solo: boolean): Worker<A>[] {
  const out: Worker<A>[] = [];
  for (const a of agents) {
    const p = phase(a);
    const ts = tasks(a);
    const running = ts.filter((t) => t.ended === undefined).length;
    if (solo ? !ts.length : p === "idle" && !ts.length) continue;
    out.push({ a, phase: p === "idle" && ts.length ? "back" : p, tasks: ts, running });
  }
  return out;
}

/** The faces shown folded: at most MAX, the rest as "+N". */
export const MAX = 3;
export const folded = <T>(ws: T[]): { shown: T[]; more: number } => ({ shown: ws.slice(0, MAX), more: Math.max(0, ws.length - MAX) });

/** The message an agent's turn is writing now: its last one being written or at work. */
export function writingOf(msgs: Msg[], session: string): Msg | undefined {
  for (let i = msgs.length - 1; i >= 0; i--) {
    const m = msgs[i]!;
    if (m.author.kind === "agent" && m.author.session === session && (m.writing || m.work?.running)) return m;
  }
  return undefined;
}

/** A timer's words: m:ss, h:mm:ss past an hour. */
export function clockOf(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, "0");
  return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}
