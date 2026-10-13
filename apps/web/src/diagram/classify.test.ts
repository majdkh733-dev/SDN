import { describe, expect, it } from "vitest";
import { classifySymbol, referenceIcon, suggestDevices } from "./classify";

describe("classifySymbol", () => {
  it("keeps reference artwork roles distinct", () => {
    expect(classifySymbol({ vertex: true, style: "shape=mxgraph.cisco19.rect;prIcon=l2_switch;fillColor=#D5E8D4", label: "", width: 95, height: 95 })).toBe("device");
    expect(classifySymbol({ vertex: true, style: "shape=image;image=data:image/png,abc", label: "", width: 180, height: 70 })).toBe("logo");
    expect(classifySymbol({ vertex: true, style: "text;html=1;fontFamily=Verdana", label: "Access Layer", width: 140, height: 30 })).toBe("label");
    expect(classifySymbol({ vertex: true, style: "image=img/cloud.svg;shape=image", label: "", width: 80, height: 50 })).toBe("cloud");
    expect(classifySymbol({ vertex: false, style: "endArrow=none;html=1;", label: "", width: 0, height: 0 })).toBe("link");
    expect(classifySymbol({ vertex: false, style: "strokeWidth=7;html=1;", label: "", width: 0, height: 0 })).toBe("boundary");
    expect(referenceIcon("shape=mxgraph.cisco19.rect;prIcon=l2_switch;fillColor=#D5E8D4")).toBe("l2 switch");
  });
});

describe("suggestDevices", () => {
  const devices = [
    { id: "a", name: "riyadh-dc1-sw-1", management_endpoint: "sim://riyadh-dc1-sw-1", serial: "SIM-1", site_id: "s" },
    { id: "b", name: "core-sw", management_endpoint: "10.1.1.5", serial: "", site_id: "s" },
  ];

  it("suggests an exact address and does not fuzzy-match a different name", () => {
    expect(suggestDevices("uplink 10.1.1.5", devices)).toEqual([{ deviceId: "b", evidence: "management-address", ambiguous: false }]);
    expect(suggestDevices("AP12 wifi", devices)).toEqual([]);
  });

  it("marks two exact name hits as review required", () => {
    const twins = [
      { id: "a", name: "Core", management_endpoint: "", serial: "", site_id: "s" },
      { id: "b", name: "core", management_endpoint: "", serial: "", site_id: "s" },
    ];
    const hits = suggestDevices("Core", twins);
    expect(hits).toHaveLength(2);
    expect(hits.every((item) => item.ambiguous)).toBe(true);
  });
});
