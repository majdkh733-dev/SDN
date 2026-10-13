// Regions follow the thick site borders in the recovered mxGraph model.
// Labels that exist as text are copied exactly. Logo-only names are the
// artwork in those borders; the selector does not replace the artwork.
export type DiagramRegion = {
  id: string;
  label: string;
  labelSource: "text" | "logo";
  x: number;
  y: number;
  width: number;
  height: number;
};

export const diagramRegions: DiagramRegion[] = [
  { id: "utec", label: "UTEC", labelSource: "logo", x: -1650, y: -1040, width: 2510, height: 2246 },
  { id: "ussg", label: "USSG", labelSource: "logo", x: 980, y: -1040, width: 2521, height: 2250 },
  { id: "epoxy", label: "Epoxy", labelSource: "logo", x: -1650, y: 1206, width: 2510, height: 532 },
  { id: "lv", label: "LV", labelSource: "logo", x: 980, y: 1210, width: 2521, height: 525 },
  { id: "pakistan-office", label: "pakistan Office", labelSource: "text", x: -1650, y: 1738, width: 2510, height: 500 },
  { id: "ss", label: "SS", labelSource: "logo", x: 980, y: 1735, width: 2521, height: 509 },
  { id: "closed-heat", label: "Closed Heat Warehouse", labelSource: "logo", x: -1650, y: 2238, width: 2510, height: 650 },
  { id: "al-kharj", label: "Al Kharj Warehouse", labelSource: "logo", x: 980, y: 2244, width: 2521, height: 628 },
];

export function plainLabel(value: string) {
  return value
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&#39;/g, "'")
    .replace(/&quot;/g, "\"")
    .replace(/\s+/g, " ")
    .trim();
}
