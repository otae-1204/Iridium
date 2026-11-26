// "#n" in a channel's words, a key to message n (lib.ts refSpans).
import { describe, expect, test } from "vitest";
import { linkNicks, nickParts, refSpans, seqOfURL } from "../src/chat/channel/lib";

const seqs = (text: string, end = 500) => refSpans(text, end).map((s) => s.seq);

describe("refSpans", () => {
  test("numbers standing by themselves, side by side", () => {
    expect(seqs("看 #462")).toEqual([462]);
    expect(seqs("#130/#147 漏掉的")).toEqual([130, 147]);
    expect(seqs("#70、#74 定稿")).toEqual([70, 74]);
    expect(seqs("(#462)，#1。")).toEqual([462, 1]);
    expect(seqs("（#9）")).toEqual([9]);
    expect(seqs("#12, #13.")).toEqual([12, 13]);
  });
  test("where it is the message's own: spans its text", () => {
    const [s] = refSpans("ok #42 here", 100);
    expect(s).toEqual({ from: 3, to: 6, seq: 42 });
  });
  test("none in code, an address, a colour, a unit, a heading, an entity, a word", () => {
    expect(seqs("`#12` and\n```\n#13\n```\n#14")).toEqual([14]);
    expect(seqs("https://github.com/x/y/pull/3#123 and www.a.b/#5")).toEqual([]);
    expect(seqs("color #333a #12px #fff #1e1e1e")).toEqual([]);
    expect(seqs("# 123\n## 5")).toEqual([]);
    expect(seqs("&#123; a#12 _#3 ##4")).toEqual([]);
    expect(seqs("[see #12](https://x) then #13")).toEqual([13]);
    expect(seqs("v1.#2 #1.5")).toEqual([]);
    expect(seqs("#0 #012")).toEqual([]);
  });
  test("not a PR's or an issue's, nor those run on from it", () => {
    expect(seqs("PR #75 和 pr #76、#77，然后 #78")).toEqual([78]);
    expect(seqs("Issue #3 / issues #4 / pull #5 / PR#6")).toEqual([]);
    expect(seqs("PR #76 已合，看 #77")).toEqual([77]);
  });
  test("only those the channel has", () => {
    expect(seqs("#1 #500 #501", 500)).toEqual([1, 500]);
    expect(seqs("#1", 0)).toEqual([]);
  });
});

describe("in the words", () => {
  test("a person's: plain, nicks and numbers in order", () => {
    expect(nickParts("@a 看 #3", ["a"], 10)).toEqual([{ nick: "a" }, " 看 ", { seq: 3 }]);
    expect(nickParts("@a 看 #3", ["a"])).toEqual([{ nick: "a" }, " 看 #3"]);
  });
  test("an agent's Markdown: links", () => {
    const md = linkNicks("@a 见 #3，`#4`", ["a"], 10);
    expect(md).toBe("[@a](ir:member/a) 见 [#3](ir:seq/3)，`#4`");
    expect(seqOfURL("ir:seq/3")).toBe(3);
    expect(seqOfURL("ir:seq/x")).toBeUndefined();
  });
});
