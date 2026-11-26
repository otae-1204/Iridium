// The link's signal (src/signal.ts): how many bars and in what tone by the round trip.
import { expect, test } from "vitest";
import { BARS, signalOf } from "../src/signal";

test("bars by the round trip, at each edge", () => {
  const at = (ms: number) => {
    const s = signalOf(true, ms);
    return s.kind === "lit" ? [s.bars, s.tone] : s.kind;
  };
  expect(at(0)).toEqual([5, "run"]);
  expect(at(200)).toEqual([5, "run"]);
  expect(at(201)).toEqual([4, "run"]);
  expect(at(400)).toEqual([4, "run"]);
  expect(at(401)).toEqual([3, "need"]);
  expect(at(800)).toEqual([3, "need"]);
  expect(at(801)).toEqual([2, "need"]);
  expect(at(1500)).toEqual([2, "need"]);
  expect(at(1501)).toEqual([1, "err"]);
  expect(at(60000)).toEqual([1, "err"]);
});

test("cut off, whatever the last round trip; not measured yet, waiting", () => {
  expect(signalOf(false, 90)).toEqual({ kind: "off" });
  expect(signalOf(false, null)).toEqual({ kind: "off" });
  expect(signalOf(null, null)).toEqual({ kind: "wait" });
  expect(signalOf(true, null)).toEqual({ kind: "wait" });
  expect(signalOf(null, 90)).toEqual({ kind: "lit", bars: 5, tone: "run" });
});

test("five bars, rising, the tallest within the 12px box", () => {
  expect(BARS).toEqual([3.2, 4.8, 6.6, 8.4, 10.2]);
});
