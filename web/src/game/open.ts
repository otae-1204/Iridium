// The ⋯ menu's Game (kumo, 2026-11-26): whether it is open. Only this is in the page's first
// load; the layer and the game itself (overlay.tsx, engine.ts, game.ts) come as it is opened.
import { useSyncExternalStore } from "react";

let open = false;
const subs = new Set<() => void>();
const set = (on: boolean) => {
  if (on === open) return;
  open = on;
  subs.forEach((f) => f());
};
export const openGame = () => set(true);
/** The layer gone, played out (overlay.tsx). */
export const closeGame = () => set(false);
export const useGameOpen = () => useSyncExternalStore((f) => (subs.add(f), () => void subs.delete(f)), () => open);
