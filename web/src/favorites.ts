// Favorites (kumo, 2026-11-26): each person's one list of channels (a session's is its own),
// at the top of the list of sessions while it holds any: none at first, it comes with the first
// one favoured (a row's menu) and goes with the last one taken off. One level, nothing made in it;
// a channel there stays in its own group too. Kept by the server under the login, so it follows the
// person to every device (internal/server/favorites.go); another's changes nothing of mine.
import { keepPreviousData } from "@tanstack/react-query";
import { useSyncExternalStore } from "react";
import { api, missing } from "./api/client";
import { useQuery } from "./api/query";
import { t } from "./i18n";
import { useCarried } from "./store";
import { errText, toast } from "./toast";
import type { Edge } from "./reorder";

/** The list as changed here, till the server has said it back: drawn in place of what it says. */
let mine: string[] | null = null;
/** The list the server last said (useFavorites keeps it). */
let known: string[] = [];
/** The changes, one after another: the last one put is what stays. */
let queue: Promise<unknown> = Promise.resolve();
const subs = new Set<() => void>();
const emit = () => subs.forEach((f) => f());
const subscribe = (f: () => void) => (subs.add(f), () => void subs.delete(f));
const NONE: string[] = [];

/** Asked again as anyone's favorites change: the carried "favorites", a count. */
const favQ = (v: number) => ({ queryKey: ["favorites", v], queryFn: () => api.favorites().then((f) => f.channels ?? NONE).catch((e) => (missing(e) ? NONE : Promise.reject(e))), staleTime: 30_000 });

/** My favorites, by channel id, in my order. */
export function useFavorites(): string[] {
  const v = useCarried<number>("favorites") ?? 0;
  const q = useQuery({ ...favQ(v), placeholderData: keepPreviousData });
  known = q.data ?? NONE;
  return useSyncExternalStore(subscribe, () => mine) ?? known;
}

const now = () => mine ?? known;

/** Makes my favorites list: drawn so at once, then put; a failed put drawn back as it was. */
function put(list: string[]) {
  mine = list;
  emit();
  queue = queue.then(() =>
    api.putFavorites(list).then(
      (f) => {
        known = f.channels ?? NONE;
        if (mine === list) (mine = null), emit();
      },
      (e: unknown) => {
        if (mine === list) (mine = null), emit();
        toast(t("err.action"), { error: true, description: errText(e) });
      },
    ),
  );
}

export const isFavorite = (id: string) => now().includes(id);

/** Favoured (last of them) or taken off. */
export const setFavorite = (id: string, on: boolean) => {
  const was = now();
  if (on === was.includes(id)) return;
  put(on ? [...was, id] : was.filter((x) => x !== id));
};

/** id put at edge of target among the favorites: moved, or favoured there if it was not. */
export function placeFavorite(id: string, target: string, edge: Edge) {
  put(favoriteAt(now(), id, target, edge));
}

/** The list with id at edge of target (id moved, or added there). */
export function favoriteAt(list: string[], id: string, target: string, edge: Edge): string[] {
  const rest = list.filter((x) => x !== id);
  const j = rest.indexOf(target);
  if (j < 0) return [...rest, id];
  rest.splice(j + (edge === "bottom" ? 1 : 0), 0, id);
  return rest;
}
