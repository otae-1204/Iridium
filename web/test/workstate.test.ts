// The work bar's state (src/chat/channel/workstate.ts): 回复中 while a turn runs, 后台 while only
// background tasks do, idle else; a task ended stays a moment marked done; who the bar shows.
import { describe, expect, test } from "vitest";
import type { Msg } from "../src/chat/channel/lib";
import { bgLaunch } from "../src/chat/fold";
import type { Session } from "../src/api/types.gen";
import { clockOf, DONE_FOR, folded, looseSubs, phaseOf, settle, taskKind, workers, writingOf, type Phase, type Shown } from "../src/chat/channel/workstate";

describe("phaseOf", () => {
  test("a turn going on is 回复中, its tasks or not", () => {
    for (const status of ["running", "compacting", "waiting"]) {
      expect(phaseOf({ status, tasks: [] })).toBe("reply");
      expect(phaseOf({ status, tasks: [{ id: "t" }] })).toBe("reply");
    }
  });
  test("its turn over, tasks running: 后台; none: idle", () => {
    expect(phaseOf({ status: "idle", tasks: [{ id: "t" }] })).toBe("back");
    expect(phaseOf({ status: "idle", tasks: [] })).toBe("idle");
    expect(phaseOf({ status: "idle", tasks: null })).toBe("idle");
    expect(phaseOf({ status: "error", tasks: [{ id: "t" }] })).toBe("back");
    expect(phaseOf(undefined)).toBe("idle");
  });
  test("starting is no turn yet", () => {
    expect(phaseOf({ status: "starting", tasks: [] })).toBe("idle");
  });
});

test("taskKind: Claude's, Codex's and dsh's sorts", () => {
  expect(taskKind({ type: "local_agent" })).toBe("agent");
  expect(taskKind({ type: "subagent" })).toBe("agent");
  expect(taskKind({ type: "local_bash" })).toBe("shell");
  expect(taskKind({ type: "bash" })).toBe("shell");
  expect(taskKind({ type: "local_workflow" })).toBe("task");
  expect(taskKind({})).toBe("task");
});

describe("settle", () => {
  test("one ended stays DONE_FOR marked done, then goes; the order stays", () => {
    const a = { id: "a", since: 1 };
    const b = { id: "b", since: 2 };
    let l: Shown[] = settle([], [a, b], 0);
    expect(l.map((x) => x.id)).toEqual(["a", "b"]);
    l = settle(l, [b], 1000);
    expect(l).toEqual([{ ...a, ended: 1000 }, b]);
    l = settle(l, [b], 1000 + DONE_FOR - 1);
    expect(l.map((x) => [x.id, x.ended])).toEqual([
      ["a", 1000],
      ["b", undefined],
    ]);
    l = settle(l, [b], 1000 + DONE_FOR);
    expect(l.map((x) => x.id)).toEqual(["b"]);
  });
  test("a new one goes at the end; what a running one says now is taken", () => {
    const l = settle([{ id: "a", steps: 1 }], [{ id: "c" }, { id: "a", steps: 3 }], 5);
    expect(l).toEqual([{ id: "a", steps: 3 }, { id: "c" }]);
  });
  test("settled again at the same time, the same", () => {
    const once = settle([{ id: "a" }], [], 7);
    expect(settle(once, [], 7)).toEqual(once);
  });
});

describe("workers", () => {
  const agents = [{ session: "x" }, { session: "y" }, { session: "z" }];
  const of = (p: Record<string, Phase>, t: Record<string, Shown[]>) => [(a: { session: string }) => p[a.session] ?? "idle", (a: { session: string }) => t[a.session] ?? []] as const;
  test("a channel: those in a turn or with tasks, in its order", () => {
    const [p, t] = of({ x: "back", z: "reply" }, { x: [{ id: "1" }, { id: "2", ended: 1 }] });
    const ws = workers(agents, p, t, false);
    expect(ws.map((w) => [w.a.session, w.phase, w.running])).toEqual([
      ["x", "back", 1],
      ["z", "reply", 0],
    ]);
  });
  test("one whose last task just ended is still shown, as background", () => {
    const [p, t] = of({}, { y: [{ id: "1", ended: 1 }] });
    expect(workers(agents, p, t, false).map((w) => [w.a.session, w.phase, w.running])).toEqual([["y", "back", 0]]);
  });
  test("a chat of one agent: shown only for its tasks", () => {
    const one = [{ session: "x" }];
    expect(workers(one, ...of({ x: "reply" }, {}), true)).toEqual([]);
    expect(workers(one, ...of({ x: "reply" }, { x: [{ id: "1" }] }), true).map((w) => w.phase)).toEqual(["reply"]);
    expect(workers(one, ...of({ x: "back" }, { x: [{ id: "1" }] }), true).map((w) => w.phase)).toEqual(["back"]);
  });
  test("three faces on a computer, two on a phone, the rest only counted", () => {
    expect(folded([1, 2])).toEqual({ shown: [1, 2], more: 0 });
    expect(folded([1, 2, 3, 4, 5])).toEqual({ shown: [1, 2, 3], more: 2 });
    expect(folded([1, 2, 3], 2)).toEqual({ shown: [1, 2], more: 1 });
  });
});

describe("looseSubs: the subagents at work no task stands for", () => {
  const sub = (id: string, parent: string, x: Partial<Session> = {}) => ({ id, parent, subagent: true, title: `t-${id}`, createdAt: 5, ...x }) as Session;
  const all = () => true;
  test("under the agent they are of; a sub of the channel's own session under its first agent", () => {
    const list = [sub("s1", "b"), sub("s2", "ch"), sub("s3", "elsewhere"), { id: "b", subagent: false } as Session];
    const m = looseSubs(list, "ch", ["a", "b"], () => [], all);
    expect([...m.entries()].map(([k, v]) => [k, v.map((x) => [x.id, x.type, x.description, x.since])])).toEqual([
      ["b", [["s1", "subagent", "t-s1", 5]]],
      ["a", [["s2", "subagent", "t-s2", 5]]],
    ]);
  });
  test("one a task stands for (by its call, or its id), or not at work, is left out", () => {
    const list = [sub("s1", "a", { call: "c1" }), sub("s2", "a"), sub("s3", "a")];
    const m = looseSubs(list, "ch", ["a"], () => [{ id: "t", call: "c1" }, { id: "s2" }], (s) => s.id !== "s3");
    expect(m.size).toBe(0);
  });
  test("no agents, none", () => {
    expect(looseSubs([sub("s1", "ch")], "ch", [], () => [], all).size).toBe(0);
  });
  test("with workers: an agent idle but a sub at work counts as background", () => {
    const loose = looseSubs([sub("s1", "y")], "ch", ["x", "y"], () => [], all);
    const ws = workers([{ session: "x" }, { session: "y" }], () => "idle", (a) => loose.get(a.session) ?? [], false);
    expect(ws.map((w) => [w.a.session, w.phase, w.running])).toEqual([["y", "back", 1]]);
  });
});

test("writingOf: the agent's last message being written or at work", () => {
  const m = (n: number, session: string, x: Partial<Msg> = {}): Msg => ({ n, text: "", files: [], author: { kind: "agent", session, nick: session, name: "c" }, ...x });
  const msgs = [m(1, "x", { writing: true }), m(2, "y", { work: { steps: 1, running: true, from: 0 } }), m(3, "x"), m(4, "x", { work: { steps: 2, running: false, from: 0 } })];
  expect(writingOf(msgs, "y")?.n).toBe(2);
  expect(writingOf(msgs, "x")?.n).toBe(1);
  expect(writingOf(msgs, "z")).toBeUndefined();
});

test("clockOf", () => {
  expect(clockOf(0)).toBe("0:00");
  expect(clockOf(42_400)).toBe("0:42");
  expect(clockOf(192_000)).toBe("3:12");
  expect(clockOf(3_723_000)).toBe("1:02:03");
  expect(clockOf(-5)).toBe("0:00");
});

test("bgLaunch: a call sent to the background", () => {
  expect(bgLaunch({ name: "Agent", input: JSON.stringify({ description: "d", run_in_background: true }) })).toBe("agent");
  expect(bgLaunch({ name: "Agent", input: JSON.stringify({ description: "d" }) })).toBeUndefined();
  expect(bgLaunch({ name: "Bash", input: JSON.stringify({ command: "npm run build", run_in_background: true }) })).toBe("shell");
  expect(bgLaunch({ name: "Bash", input: JSON.stringify({ command: "ls" }) })).toBeUndefined();
  expect(bgLaunch({ name: "Monitor", input: "{}" })).toBe("shell");
});
