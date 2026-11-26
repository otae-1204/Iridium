// Backspace in an empty box takes the attachments, two presses each (src/chat/bsattach.ts).
import { describe, expect, test } from "vitest";
import { press, type Press } from "../src/chat/bsattach";

const keys = ["a", "b"];
const bs = (o: Partial<Press> = {}): Press => ({ key: "Backspace", words: "", composing: false, keys, ...o });

describe("press", () => {
  test("words in the box: not ours", () => {
    expect(press(null, bs({ words: "hi" }))).toEqual({ marked: null, ours: false });
    expect(press("b", bs({ words: "x" }))).toEqual({ marked: null, ours: false });
  });
  test("an input method composing: not ours, the mark let go", () => {
    expect(press(null, bs({ composing: true }))).toEqual({ marked: null, ours: false });
    expect(press("b", bs({ composing: true }))).toEqual({ marked: null, ours: false });
  });
  test("nothing attached: not ours", () => {
    expect(press(null, bs({ keys: [] })).ours).toBe(false);
  });
  test("one press marks the last, takes nothing", () => {
    expect(press(null, bs())).toEqual({ marked: "b", ours: true });
  });
  test("two take it; two more the next", () => {
    const one = press(null, bs());
    const two = press(one.marked, bs());
    expect(two).toEqual({ marked: null, take: "b", ours: true });
    const three = press(two.marked, bs({ keys: ["a"] }));
    expect(three).toEqual({ marked: "a", ours: true });
    expect(press(three.marked, bs({ keys: ["a"] }))).toEqual({ marked: null, take: "a", ours: true });
  });
  test("another key between lets the mark go: the next Backspace only marks", () => {
    const one = press(null, bs());
    const other = press(one.marked, bs({ key: "a" }));
    expect(other).toEqual({ marked: null, ours: false });
    expect(press(other.marked, bs())).toEqual({ marked: "b", ours: true });
  });
  test("held down: nothing taken", () => {
    expect(press("b", bs({ repeat: true }))).toEqual({ marked: "b", ours: true });
    expect(press(null, bs({ repeat: true }))).toEqual({ marked: null, ours: true });
  });
});
