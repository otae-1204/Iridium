import { expect, test } from "vitest";
import { keepShut, SHUT_KEY, shutKept } from "../src/sideshut";

/** A Storage of a plain map. */
function mem(): Storage {
  const m = new Map<string, string>();
  return {
    get length() {
      return m.size;
    },
    clear: () => m.clear(),
    getItem: (k) => m.get(k) ?? null,
    key: (i) => [...m.keys()][i] ?? null,
    removeItem: (k) => void m.delete(k),
    setItem: (k, v) => void m.set(k, String(v)),
  };
}

test("the sidebar's shut kept on this device, open when nothing is kept", () => {
  const s = mem();
  const at = () => s;
  expect(shutKept(at)).toBe(false);
  keepShut(true, at);
  expect(s.getItem(SHUT_KEY)).toBe("1");
  expect(shutKept(at)).toBe(true);
  keepShut(false, at);
  expect(s.getItem(SHUT_KEY)).toBeNull();
  expect(shutKept(at)).toBe(false);
});

test("no storage at all: open, and nothing thrown", () => {
  const none = () => {
    throw new Error("denied");
  };
  expect(shutKept(none)).toBe(false);
  expect(() => keepShut(true, none)).not.toThrow();
});
