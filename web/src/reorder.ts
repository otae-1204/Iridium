// The list's order by hand (the user, 2026-11-09): a group's row dragged among all groups' rows, a
// folder among those in its group, a session among those beside it (in its group's root or its
// folder). Pinned ones are ordered apart from the rest; one dropped on a pinned row is
// pinned, on another unpinned. A folder pinned is above the sessions at its group's root; one
// dropped on any of those is pinned, last of the pinned. A session or folder dragged keeps the key
// it was dropped at (Session.order, a folder's mark in the carried "guilds") in place of its last activity: it
// stays where it was put while those never dragged are still sorted by when they last worked.
// With a mouse it is dragged; a finger holds the row (its menu opens), then moves it (touchDrag)
// (kumo, 2026-11-26: no 上移/下移 in a group's menu).
import { useSyncExternalStore } from "react";
import type { Session } from "./api/types.gen";
import { NO_GUILDS, placesOf, useGuilds, type Places } from "./guilds";
import { folderKey, type Folder } from "./tree";

/** What is dragged: a group (its ID), a folder or a session, and the rows it may be dropped among. */
export type Dragged = { kind: "ws" | "f" | "s"; id: string; lane: string };
/** The row's edge it goes to: above it or under it. */
export type Edge = "top" | "bottom";

/** The row a drop would go to now (its mark, dropMark), the row the line is drawn at (that one,
 *  or the one the drop lands above) and its edge. */
export type Over = { at: string; line: string; edge: Edge };
/** A drop a row takes of another kind, or from another lane (lane ANY: from any), and the row
 *  whose top it lands at (dropMark), or its foot (edge). */
export type Also = { kind: Dragged["kind"]; lane: string; drop: (d: Dragged) => void; line: string; edge?: Edge };
/** An Also's lane: what is dragged from any lane. */
export const ANY = "*";

let dragged: Dragged | null = null;
let over: Over | null = null;
const subs = new Set<() => void>();
const subscribe = (f: () => void) => (subs.add(f), () => void subs.delete(f));
const emit = () => subs.forEach((f) => f());
const setOver = (o: Over | null) => {
  if (o?.at === over?.at && o?.line === over?.line && o?.edge === over?.edge) return;
  over = o;
  emit();
};

/** The list's scrolling as a drag nears its top or foot, stopped as the drag ends. */
let stopScroll: (() => void) | null = null;

/** A drag begun: with a mouse, row is the row taken (the list it is in scrolls as the pointer
 *  nears its top or foot, kumo, 2026-11-26); a finger's (touchDrag) scrolls its own. */
export const startDrag = (d: Dragged, row?: HTMLElement) => {
  dragged = d;
  stopScroll?.();
  stopScroll = null;
  const list = row && scroller(row);
  if (list) {
    let at: { x: number; y: number } | null = null;
    const track = (e: DragEvent) => void (at = { x: e.clientX, y: e.clientY });
    addEventListener("dragover", track, true);
    const stop = edgeScroll(list, () => at);
    stopScroll = () => (stop(), removeEventListener("dragover", track, true));
  }
  emit();
};
export const endDrag = () => {
  stopScroll?.();
  stopScroll = null;
  dragged = null;
  over = null;
  emit();
};

/** What the list takes, round its rows: over no row a drop goes to (none took the dragover) or
 *  out of the list, no line. Not on leaving a row: the line would blink between two. */
export const listDrop = () => ({
  onDragOver: (e: React.DragEvent<HTMLElement>) => void (!e.defaultPrevented && setOver(null)),
  onDragLeave: (e: React.DragEvent<HTMLElement>) => void (!e.currentTarget.contains(e.relatedTarget as Node | null) && setOver(null)),
});

/** The row a drop would go to now (the list draws one line, gliding from row to row). */
export const useOver = () => useSyncExternalStore(subscribe, () => over);
/** That the row of id is the one dragged now: it fades while it is. */
export const useDragged = (kind: Dragged["kind"], id: string) => useSyncExternalStore(subscribe, () => dragged?.kind === kind && dragged.id === id);
/** The row's mark the list finds it by, to draw the line at: data-place. */
export const dropMark = (d: Dragged) => `${d.kind}:${d.lane}:${d.id}`;

/** A row's drop, found by its mark (data-place) under a finger (touchDrag): as it was last drawn. */
type Target = { pick: () => "own" | "also" | null; aim: (how: "own" | "also", r: DOMRect, y: number) => Over; land: (how: "own" | "also", d: Dragged, edge: Edge) => void };
const targets = new Map<string, Target>();

/** What a row takes to be dropped on: by one of its kind and lane (any lane: anyLane, a
 *  favourite's, taking a session from anywhere), not itself; also, by one of another kind (a folder
 *  on a session at its group's root). */
export function dropTarget(kind: Dragged["kind"], id: string, lane: string, drop: (d: Dragged, edge: Edge) => void, also?: Also, anyLane = false) {
  const at = dropMark({ kind, id, lane });
  const t: Target = {
    pick: () => {
      const d = dragged;
      if (d?.kind === kind && (d.lane === lane || anyLane) && d.id !== id) return "own";
      if (also && d?.kind === also.kind && (d.lane === also.lane || also.lane === ANY)) return "also";
      return null;
    },
    aim: (how, r, y) => (how === "also" ? { at, line: also!.line, edge: also!.edge ?? "top" } : { at, line: at, edge: y < r.top + r.height / 2 ? "top" : "bottom" }),
    land: (how, d, edge) => (how === "also" ? also!.drop(d) : drop(d, edge)),
  };
  targets.set(at, t);
  return {
    "data-place": at,
    onDragOver: (e: React.DragEvent<HTMLElement>) => {
      const how = t.pick();
      if (!how) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
      setOver(t.aim(how, e.currentTarget.getBoundingClientRect(), e.clientY));
    },
    onDrop: (e: React.DragEvent<HTMLElement>) => {
      const how = t.pick();
      if (!how || over?.at !== at) return;
      e.preventDefault();
      const d = dragged!;
      const edge = over.edge;
      endDrag();
      t.land(how, d, edge);
    },
  };
}

/** The nearest box above el that scrolls up and down (the sidebar's list). */
function scroller(el: HTMLElement): HTMLElement | null {
  for (let p = el.parentElement; p; p = p.parentElement) {
    const o = getComputedStyle(p).overflowY;
    if ((o === "auto" || o === "scroll") && p.scrollHeight > p.clientHeight) return p;
  }
  return null;
}

/** The edge band a drag scrolls the list in: 56px, a quarter of a short list. The speed grows
 *  with how deep in the band the pointer is (out past the list's edge: all of it), up to MAX px a
 *  millisecond, and from nothing over RAMP ms after it came in, so a pointer that passes the band
 *  on its way, or rests in it, does not throw the list to its end. */
const BAND = 56;
const MAX = 1.2;
const RAMP = 400;

/** Scrolls list while the point where() gives (a pointer's, null: none yet) is in its top or
 *  foot band and within its width; moved: after each step that scrolled it. Stopped by what it
 *  gives back. */
export function edgeScroll(list: HTMLElement, where: () => { x: number; y: number } | null, moved?: () => void): () => void {
  let since = 0;
  let last = 0;
  let frame = requestAnimationFrame(function tick(now) {
    const p = where();
    const b = list.getBoundingClientRect();
    const band = Math.min(BAND, b.height / 4);
    const depth = !p || p.x < b.left || p.x > b.right ? 0 : p.y < b.top + band ? -Math.min(1, (b.top + band - p.y) / band) : p.y > b.bottom - band ? Math.min(1, (p.y - b.bottom + band) / band) : 0;
    if (!depth) since = 0;
    else {
      since ||= now;
      const dt = Math.min(32, now - (last || now));
      const v = Math.sign(depth) * MAX * depth * depth * Math.min(1, (now - since) / RAMP);
      const was = list.scrollTop;
      list.scrollTop += v * dt;
      if (list.scrollTop !== was) moved?.();
    }
    last = now;
    frame = requestAnimationFrame(tick);
  });
  return () => cancelAnimationFrame(frame);
}

/** A finger's drag: row held (its menu opened, ui.tsx useLongPress), then moved. The row's copy
 *  goes with the finger, just above it (the line where it would land drawn under it, as a mouse's is), the list scrolled
 *  near its top or foot; let go over a row that takes it, it lands there as a mouse's drop. */
export function touchDrag(d: Dragged, row: HTMLElement, from: { x: number; y: number }, pointerId: number) {
  startDrag(d);
  const r = row.getBoundingClientRect();
  const ghost = row.cloneNode(true) as HTMLElement;
  ghost.removeAttribute("data-place");
  ghost.setAttribute("aria-hidden", "true");
  Object.assign(ghost.style, {
    position: "fixed",
    left: `${r.left}px`,
    top: `${r.top}px`,
    width: `${r.width}px`,
    height: `${r.height}px`,
    margin: "0",
    pointerEvents: "none",
    zIndex: "60",
    borderRadius: "8px",
    background: "var(--paper)",
    boxShadow: "var(--shadow-float)",
  });
  document.body.append(ghost);
  const list = scroller(row);
  let x = from.x;
  let y = from.y;
  const hit = () => {
    ghost.style.transform = `translateY(${y - r.bottom - 16}px)`;
    const el = document.elementFromPoint(x, y)?.closest<HTMLElement>("[data-place]");
    const t = el && targets.get(el.dataset.place!);
    const how = t?.pick();
    setOver(t && how ? t.aim(how, el.getBoundingClientRect(), y) : null);
  };
  const stop = list ? edgeScroll(list, () => ({ x, y }), hit) : () => {};
  const move = (e: PointerEvent) => {
    if (e.pointerId !== pointerId) return;
    x = e.clientX;
    y = e.clientY;
    hit();
  };
  // The page not scrolled by the finger while it drags.
  const still = (e: TouchEvent) => void (e.cancelable && e.preventDefault());
  const end = (e: PointerEvent) => {
    if (e.pointerId !== pointerId) return;
    stop();
    ghost.remove();
    removeEventListener("pointermove", move);
    removeEventListener("pointerup", end);
    removeEventListener("pointercancel", end);
    removeEventListener("touchmove", still);
    const o = over;
    const t = o && targets.get(o.at);
    const how = e.type === "pointerup" ? t?.pick() : null;
    endDrag();
    if (o && t && how) t.land(how, d, o.edge);
  };
  addEventListener("pointermove", move);
  addEventListener("pointerup", end);
  addEventListener("pointercancel", end);
  addEventListener("touchmove", still, { passive: false });
  hit();
}

/** What a session is sorted by: where it was dragged to, else its last activity (as the server's
 *  session.List). */
export const sortKey = (s: Session) => s.order || s.updatedAt;

/** The list sorted as the server sorts it: pinned first, then by sortKey, the latest first. */
export const sortList = (list: Session[]) => [...list].sort((a, b) => Number(!!b.pinned) - Number(!!a.pinned) || sortKey(b) - sortKey(a));

/** The key one goes to, dropped at edge of the row at j in zone (those it goes among, as listed,
 *  itself left out; key: what each is sorted by, the latest first): between its neighbours there,
 *  now at the top, a second under the last at the foot. */
function keyAt<T>(zone: T[], j: number, key: (x: T) => number, now: number) {
  const prev = zone[j - 1];
  const next = zone[j];
  return prev && next ? (key(prev) + key(next)) / 2 : next ? Math.max(now, key(next) + 1) : key(prev!) - 1000;
}

/** Where session s goes, dropped at edge of target among sibs (the sessions beside it, as listed):
 *  the key it takes, and pinned as the target is. */
export function placeSession(sibs: Session[], s: Session, target: Session, edge: Edge, now = Date.now()): { order: number; pinned: boolean } {
  const pinned = !!target.pinned;
  const zone = sibs.filter((x) => !!x.pinned === pinned && x.id !== s.id);
  const j = zone.findIndex((x) => x.id === target.id) + (edge === "bottom" ? 1 : 0);
  return { order: keyAt(zone, j, sortKey, now), pinned };
}

/** Where folder f goes, dropped at edge of target among sibs (the folders of its group, as
 *  listed): the key it takes, and pinned as the target is. */
export function placeFolder(sibs: Folder[], f: Folder, target: Folder, edge: Edge, places: Places, now = Date.now()): { order: number; pinned: boolean } {
  const pinned = !!target.pinned;
  const zone = sibs.filter((x) => !!x.pinned === pinned && x.path !== f.path);
  const j = zone.findIndex((x) => x.path === target.path) + (edge === "bottom" ? 1 : 0);
  return { order: keyAt(zone, j, (x) => folderKey(x, places.order), now), pinned };
}

/** Folder f pinned last of its group's pinned (dropped on a session at its root): the key under
 *  the last, none if it is the first. */
export function pinFolder(sibs: Folder[], f: Folder, places: Places): { order?: number; pinned: true } {
  const last = sibs.filter((x) => x.pinned && x.path !== f.path).at(-1);
  return last ? { order: folderKey(last, places.order) - 1000, pinned: true } : { pinned: true };
}

/** Where the rows were put by hand, the groups' and the folders' (the carried "guilds"). */
export const usePlaces = (): Places => placesOf(useGuilds() ?? NO_GUILDS);

/** The groups' rows' keys with key moved to edge of target (its position: PATCH /v1/internal/guilds/{id}).
 *  stored: the order there was; shown: the rows as listed now (those not stored after the
 *  stored). A row stored but not listed now (none of its sessions shown) keeps its place. */
export function placeGroup(stored: string[], shown: string[], key: string, target: string, edge: Edge): string[] {
  const all = [...stored, ...shown.filter((k) => !stored.includes(k))].filter((k) => k !== key);
  all.splice(all.indexOf(target) + (edge === "bottom" ? 1 : 0), 0, key);
  return all;
}
