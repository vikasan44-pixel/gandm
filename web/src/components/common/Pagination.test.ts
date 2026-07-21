import { describe, expect, it } from "vitest";
import { compactPages } from "./Pagination";

describe("compactPages", () => {
  it("keeps the current page and both boundaries", () => {
    expect(compactPages(5, 10)).toEqual([1, "ellipsis", 4, 5, 6, "ellipsis", 10]);
  });

  it("does not emit duplicate ellipses near an edge", () => {
    expect(compactPages(1, 4)).toEqual([1, 2, "ellipsis", 4]);
    expect(compactPages(4, 4)).toEqual([1, "ellipsis", 3, 4]);
  });
});
