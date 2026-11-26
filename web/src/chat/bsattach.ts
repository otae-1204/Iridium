// Backspace in an empty box takes the attachments, the last first, two presses each (kumo,
// 2026-11-26): the first marks it, the second takes it; another key, words typed, an input
// method composing, the box left or a while gone by let the mark go. Pure, for tests;
// composer.tsx keeps what is marked.

/** How long a mark stays without a second press. */
export const MARK_FOR = 3000;

/** A key pressed in the box: key, its name; words, what the box holds; composing, an input
 *  method at work; repeat, held down; keys, the attachments in their order. */
export type Press = { key: string; words: string; composing: boolean; repeat?: boolean; keys: readonly string[] };

/** What is marked after the press, the attachment to take, if any, and whether the press was
 *  ours (the box then does nothing else with it). */
export function press(marked: string | null, p: Press): { marked: string | null; take?: string; ours: boolean } {
  if (p.composing || p.key !== "Backspace" || p.words || !p.keys.length) return { marked: null, ours: false };
  // Held down: nothing taken, a mark kept as it was.
  if (p.repeat) return { marked, ours: true };
  const last = p.keys[p.keys.length - 1]!;
  return marked === last ? { marked: null, take: last, ours: true } : { marked: last, ours: true };
}
