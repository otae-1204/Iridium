import { describe, expect, test } from "vitest";
import { favoriteAt } from "../src/favorites";

describe("favorites", () => {
  test("a favourite moved, or one from elsewhere put, at a row's edge", () => {
    expect(favoriteAt(["a", "b", "c"], "c", "a", "top")).toEqual(["c", "a", "b"]);
    expect(favoriteAt(["a", "b", "c"], "a", "c", "bottom")).toEqual(["b", "c", "a"]);
    expect(favoriteAt(["a", "b"], "x", "b", "top")).toEqual(["a", "x", "b"]);
    expect(favoriteAt(["a", "b"], "x", "gone", "top")).toEqual(["a", "b", "x"]);
  });
});
