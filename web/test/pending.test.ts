// The list's changes made here (pending.ts; docs/前端设计规范.md "侧栏状态与变更"): drawn at once,
// kept over lists the stream brings till one has them, undone only when refused; a row being made
// stands in at once and gives way to the real one without a gap or a double.
import { beforeEach, describe, expect, test } from "vitest";
import type { Session } from "../src/api/types.gen";
import { change, fromServer, MAKING, making, reset } from "../src/pending";
import { getCarried, onCarried, setCarried } from "../src/store";

const row = (id: string, more: Partial<Session> = {}): Session => ({ id, agent: "claude", cwd: "/w", status: "idle", hosted: false, live: false, updatedAt: 1, role: "", ...more }) as Session;
const list = () => getCarried<Session[]>("list") ?? [];
/** What the stream does with a list it brings (stream.ts). */
const bring = (l: Session[]) => setCarried("list", fromServer(l));
const later = () => new Promise((r) => setTimeout(r, 0));

/** Every list drawn while f runs. */
async function drawn(f: () => Promise<void>): Promise<Session[][]> {
  const out: Session[][] = [];
  const stop = onCarried((k) => k === "list" && out.push(list()));
  await f();
  stop();
  return out;
}

beforeEach(() => {
  reset();
  setCarried("list", null);
});

describe("pending", () => {
  // kumo, 2026-11-26: 移出并归档, the row came into its folder, then went.
  test("archived: never drawn back, a list from before the change brought on the way", async () => {
    bring([row("a"), row("b")]);
    let answer!: () => void;
    const seen = await drawn(async () => {
      const p = change("a", { archived: true }, () => new Promise<void>((r) => (answer = r)));
      bring([row("a"), row("b")]); // made before the change, on its way
      answer();
      await p;
      bring([row("a"), row("b")]); // still before it: the server's next list not out yet
      bring([row("a", { archived: true }), row("b")]); // the list that has it
      bring([row("a", { archived: true }), row("b")]);
    });
    for (const l of seen) expect(l.find((s) => s.id === "a")?.archived).toBe(true);
    expect(list().find((s) => s.id === "a")?.archived).toBe(true);
  });

  test("refused: drawn back as it was, the error thrown on", async () => {
    bring([row("a", { name: "old" })]);
    await expect(change("a", { name: "new" }, () => Promise.reject(new Error("no")))).rejects.toThrow("no");
    expect(list()[0]!.name).toBe("old");
    // and lists after it are the server's
    bring([row("a", { name: "old" })]);
    expect(list()[0]!.name).toBe("old");
  });

  test("moved: sorted at once as the server sorts, kept till the list has it", async () => {
    bring([row("a", { updatedAt: 3 }), row("b", { updatedAt: 2 })]);
    const p = change("b", { order: 10 }, async () => {});
    expect(list().map((s) => s.id)).toEqual(["b", "a"]);
    await p;
    bring([row("a", { updatedAt: 3 }), row("b", { updatedAt: 2 })]);
    expect(list().map((s) => s.id)).toEqual(["b", "a"]);
    bring([row("b", { updatedAt: 2, order: 10 }), row("a", { updatedAt: 3 })]);
    expect(list().map((s) => s.id)).toEqual(["b", "a"]);
  });

  // kumo, 2026-11-26: the "+" pressed, nothing for a second or more.
  test("made: its row at once, under its id once made, one row when the list has it", async () => {
    bring([row("a", { updatedAt: 1 })]);
    let made!: (id: string) => void;
    const seen = await drawn(async () => {
      const p = making({ agent: "" as Session["agent"], cwd: "/w", status: "idle", hosted: false, live: false, updatedAt: 5 } as Omit<Session, "id">, () => new Promise((r) => (made = r)));
      await later();
      expect(list()[0]!.id.startsWith(MAKING)).toBe(true);
      made("n1");
      expect(await p).toBe("n1");
      bring([row("a", { updatedAt: 1 })]); // before the server listed it
      bring([row("n1", { updatedAt: 5 }), row("a", { updatedAt: 1 })]);
    });
    // Never gone between its stand-in and itself, never twice.
    for (const l of seen) expect(l.filter((s) => s.id === "n1" || s.id.startsWith(MAKING))).toHaveLength(1);
    expect(list().map((s) => s.id)).toEqual(["n1", "a"]);
  });

  // The list may come before the answer naming the row (the server lists it apart, after).
  test("made: listed before its id is known, still one row", async () => {
    bring([row("a", { updatedAt: 1 })]);
    let made!: (id: string) => void;
    const seen = await drawn(async () => {
      const p = making({ agent: "" as Session["agent"], cwd: "/w", status: "idle", hosted: false, live: false, updatedAt: 5, bare: true } as Omit<Session, "id">, () => new Promise((r) => (made = r)));
      await later();
      bring([row("n2", { updatedAt: 5, bare: true }), row("a", { updatedAt: 1 })]);
      made("n2");
      await p;
    });
    for (const l of seen) expect(l.filter((s) => s.id === "n2" || s.id.startsWith(MAKING))).toHaveLength(1);
    expect(list().map((s) => s.id)).toEqual(["n2", "a"]);
  });

  test("not made: its row gone", async () => {
    bring([row("a")]);
    expect(await making({ agent: "" as Session["agent"], cwd: "/w", status: "idle", hosted: false, live: false, updatedAt: 5 } as Omit<Session, "id">, async () => null)).toBeNull();
    expect(list().map((s) => s.id)).toEqual(["a"]);
  });
});
