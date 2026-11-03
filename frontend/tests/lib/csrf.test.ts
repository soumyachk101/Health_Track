import { describe, expect, it } from "vitest";
import { getCookie } from "@/lib/csrf";

describe("lib/csrf", () => {
  it("getCookie is an inert no-op that always returns null", () => {
    expect(getCookie("anything")).toBeNull();
  });
});
