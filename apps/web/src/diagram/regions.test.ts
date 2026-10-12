import { describe, expect, it } from "vitest";
import { diagramRegions, plainLabel } from "./regions";

describe("reference diagram regions", () => {
  it("keeps the source site labels", () => {
    expect(diagramRegions.map((item) => item.label)).toEqual([
      "UTEC",
      "USSG",
      "Epoxy",
      "LV",
      "pakistan Office",
      "SS",
      "Closed Heat Warehouse",
      "Al Kharj Warehouse",
    ]);
  });

  it("strips draw.io html without changing the words", () => {
    expect(plainLabel("<font>pakistan Office</font>")).toBe("pakistan Office");
    expect(plainLabel("AP22 wifi&nbsp;")).toBe("AP22 wifi");
  });
});
