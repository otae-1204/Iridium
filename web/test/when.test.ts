// A press on a message's blank (src/chat/said.tsx pressWhen): only where there is no hover; a line
// of a run lights its own time and number alone, the head the row's.
import { describe, expect, test } from "vitest";
import { pressWhen } from "../src/chat/said";

function run() {
  const row = document.createElement("div");
  row.className = "group/msg";
  row.innerHTML = `<div data-lead="" class="head"></div><div class="group/line" data-lead="" data-seq="1"><p>a</p></div><div class="group/line" data-seq="2"><p>b</p></div>`;
  document.body.append(row);
  const [head, first, second] = [row.querySelector(".head")!, row.querySelector('[data-seq="1"]')!, row.querySelector('[data-seq="2"]')!];
  return { row, head, first, second };
}

describe("pressWhen", () => {
  test("with hover (a computer): nothing, however pressed", () => {
    const { second, first } = run();
    expect(pressWhen(second.firstElementChild!, false)).toBe("none");
    expect(second.hasAttribute("data-when") || first.hasAttribute("data-when")).toBe(false);
  });
  test("a finger on a later message: its own alone, the first's and the head's not", () => {
    const { first, second } = run();
    expect(pressWhen(second.firstElementChild!, true)).toBe("line");
    expect(second.hasAttribute("data-when")).toBe(true);
    expect(first.hasAttribute("data-when")).toBe(false);
    expect(pressWhen(second, true)).toBe("line");
    expect(second.hasAttribute("data-when")).toBe(false);
  });
  test("a finger on the first message: its line (the head's time shows by it), not the others", () => {
    const { first, second } = run();
    expect(pressWhen(first.firstElementChild!, true)).toBe("line");
    expect(first.hasAttribute("data-when")).toBe(true);
    expect(second.hasAttribute("data-when")).toBe(false);
  });
  test("a finger on the head: the row's", () => {
    const { head } = run();
    expect(pressWhen(head, true)).toBe("row");
  });
});
