// The link's signal (link.tsx SignalIcon, kumo + Neon, 2026-11-26), pure for tests: five bars, how
// many lit and in what tone by the round trip; cut off, all five dark under a slash; not measured
// yet, all five faint, breathing.

export type Tone = "run" | "need" | "err";
export type Signal = { kind: "lit"; bars: 1 | 2 | 3 | 4 | 5; tone: Tone } | { kind: "off" } | { kind: "wait" };

/** The bars by the round trip (ms): to 200 five green, to 400 four green, to 800 three yellow,
 *  to 1500 two yellow, beyond one red. */
export function signalOf(live: boolean | null, rtt: number | null): Signal {
  if (live === false) return { kind: "off" };
  if (rtt === null) return { kind: "wait" };
  if (rtt <= 200) return { kind: "lit", bars: 5, tone: "run" };
  if (rtt <= 400) return { kind: "lit", bars: 4, tone: "run" };
  if (rtt <= 800) return { kind: "lit", bars: 3, tone: "need" };
  if (rtt <= 1500) return { kind: "lit", bars: 2, tone: "need" };
  return { kind: "lit", bars: 1, tone: "err" };
}

/** The bars' heights (px), shortest first, in a 15×12 box. */
export const BARS = [3.5, 5.5, 7.5, 9.5, 11.5] as const;
