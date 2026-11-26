// The list's changes made here (docs/前端设计规范.md "侧栏状态与变更", kumo, 2026-11-26): drawn at
// once, and kept over every list the stream brings till that list has them: one the server made
// before the change (on its way as it was sent) does not draw the row back as it was, then
// changed again. Undone, and said, only if the server refused it. A row being made (the "+")
// stands in its place at once, under the id it will have once the server has made it.
import { api } from "./api/client";
import type { Session, SessionPatch } from "./api/types.gen";
import { sortList } from "./reorder";
import { getCarried, setCarried } from "./store";

type Change = {
  /** The rows changed, and how. */
  ids: Set<string>;
  patch: Partial<Session>;
  /** A row standing in for one being made, till the list has it; before: the rows listed as it
   *  began (a row of its folder not among them, listed before its id is known, is it). */
  add?: Session;
  before?: Set<string>;
  /** Sent and taken by the server: dropped once a list has it (or after LATE). */
  done: boolean;
};

/** How long a change taken by the server is drawn over lists that do not have it yet, at most. */
export const LATE = 10_000;
/** The id a row being made stands under till the server says its own. */
export const MAKING = "making:";

const changes = new Set<Change>();
/** The list as the server last gave it. */
let server: Session[] | undefined;

/** list with the changes made here over it. */
function over(list: Session[]): Session[] {
  if (!changes.size) return list;
  let out = list;
  let moved = false;
  for (const c of changes) {
    if (c.add && !out.some((s) => s.id === c.add!.id || isIt(c, s))) (out = [c.add, ...out]), (moved = true);
    if (c.ids.size) out = out.map((s) => (c.ids.has(s.id) ? { ...s, ...c.patch } : s));
    if ("order" in c.patch || "pinned" in c.patch) moved = true;
  }
  // Sorted again as the server sorts it, where a row moved or came.
  return moved ? sortList(out) : out;
}

/** Whether s is the row c stands in for, listed (the list may come before the answer that names
 *  it): new since c began, bare, in its folder. */
const isIt = (c: Change, s: Session) => !!c.before && c.add!.id.startsWith(MAKING) && !c.before.has(s.id) && !!s.bare && s.cwd === c.add!.cwd && (s.machine ?? "") === (c.add!.machine ?? "");

/** Whether list has what c did: each row changed as c says (or gone), the one made there. */
function has(list: Session[], c: Change): boolean {
  if (c.add) return list.some((s) => s.id === c.add!.id);
  return [...c.ids].every((id) => {
    const s = list.find((x) => x.id === id);
    return !s || Object.entries(c.patch).every(([k, v]) => none(s[k as keyof Session]) === none(v));
  });
}

/** A field's value as the server says it: "", false and nothing are all its absence. */
const none = (v: unknown) => (v === "" || v === false || v == null ? undefined : v);

const redraw = () => {
  const base = server ?? getCarried<Session[]>("list");
  if (base) setCarried("list", over(base));
};

/** The list the stream brought: the changes it has dropped, the rest drawn over it. */
export function fromServer(list: Session[]): Session[] {
  server = list;
  for (const c of changes) if (c.done && has(list, c)) changes.delete(c);
  return over(list);
}

function settle(c: Change) {
  c.done = true;
  if (server && has(server, c)) changes.delete(c);
  else setTimeout(() => changes.delete(c) && redraw(), LATE);
  redraw();
}

/** Rows ids changed as patch says, drawn so at once; send tells the server. Refused: drawn back
 *  as they were, and the error thrown on (the caller says it). */
export async function change<T>(ids: string | string[], patch: Partial<Session>, send: () => Promise<T>): Promise<T> {
  const c: Change = { ids: new Set([ids].flat()), patch, done: false };
  changes.add(c);
  redraw();
  try {
    const out = await send();
    settle(c);
    return out;
  } catch (e) {
    changes.delete(c);
    redraw();
    throw e;
  }
}

/** Sessions ids patched (archived, restored, pinned, renamed, tagged, moved): drawn so at once,
 *  each told the server (SessionPatch), as change. */
export const patchSessions = (ids: string | string[], patch: SessionPatch & Partial<Session>) =>
  change(ids, patch as Partial<Session>, () => Promise.all([ids].flat().map((id) => api.patch(id, patch))));

let made = 0;

/** A row being made (row: what it will look like), standing in the list at once; make makes it
 *  and gives its id (null: not made, said already). Made: the row stands under that id till the
 *  list has it. Not made: gone. */
export async function making(row: Omit<Session, "id">, make: () => Promise<string | null>): Promise<string | null> {
  const was = server ?? getCarried<Session[]>("list") ?? [];
  const c: Change = { ids: new Set(), patch: {}, add: { ...row, id: `${MAKING}${++made}` } as Session, before: new Set(was.map((s) => s.id)), done: false };
  changes.add(c);
  redraw();
  let id: string | null = null;
  try {
    id = await make();
  } finally {
    if (id) {
      c.add = { ...c.add!, id };
      settle(c);
    } else {
      changes.delete(c);
      redraw();
    }
  }
  return id;
}

/** For the tests: nothing pending, no list known. */
export function reset() {
  changes.clear();
  server = undefined;
}
