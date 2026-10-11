import { describe, expect, it } from "vitest";
import { confirmationMatches } from "./confirm";

describe("confirmationMatches", () => {
  it("requires the plan prefix", () => {
    expect(confirmationMatches("abcdef123456", "ABCDEF12")).toBe(true);
    expect(confirmationMatches("abcdef123456", "abcdef99")).toBe(false);
  });
});
