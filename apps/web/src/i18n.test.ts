import { describe, expect, it } from "vitest";
import { direction, translate } from "./i18n";

describe("locale", () => {
  it("switches direction and labels", () => {
    expect(direction("en")).toBe("ltr");
    expect(direction("ar")).toBe("rtl");
    expect(translate("ar", "policies")).toBe("استوديو السياسات");
    expect(translate("en", "missing")).toBe("missing");
  });
});
