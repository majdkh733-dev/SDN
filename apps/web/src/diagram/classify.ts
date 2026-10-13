import { diagramRegions, type DiagramRegion } from "./regions";

export type SymbolKind = "device" | "link" | "boundary" | "separator" | "label" | "logo" | "group" | "cloud" | "decoration";

export function classifySymbol(input: { vertex: boolean; style: string; label: string; width: number; height: number }): SymbolKind {
  const style = input.style || "";
  if (!input.vertex) {
    if (/strokeWidth=7/.test(style)) return "boundary";
    if (/dashed=1/.test(style)) return "separator";
    return "link";
  }
  if (/internet_cloud|img\/cloud|img\/vpn|shape=cloud|mxgraph\.azure\.cloud/.test(style)) return "cloud";
  if (/shape=image/.test(style) || /image=data:image/.test(style)) return "logo";
  if (/prIcon=|mxgraph\.cisco|mxgraph\.networks\.(server|firewall|nas)|mxgraph\.ios7\.misc\.wifi|mxgraph\.electrical\.transmission|mxgraph\.azure\.server/.test(style)) return "device";
  if (/swimlane|container=1/.test(style) || (/partialRectangle/.test(style) && input.width >= 400 && input.height >= 200)) return "group";
  if (/^text;|shape=text|labelBackgroundColor=/.test(style) || (input.label && input.width > 0 && input.height > 0 && input.height <= 80 && !/shape=/.test(style))) return "label";
  if (/tableRow|shape=table|partialRectangle|strokeWidth=7/.test(style)) return "decoration";
  if (input.label && !/shape=/.test(style)) return "label";
  return "decoration";
}

export function referenceIcon(style: string): string {
  const named = style.match(/prIcon=([^;]+)/);
  const shaped = style.match(/mxgraph\.(?:cisco(?:\.\w+)?|networks|ios7\.misc|electrical\.transmission|azure)\.([a-z0-9_]+)/i);
  const raw = named?.[1] || shaped?.[1] || "";
  return raw.replace(/_/g, " ");
}

export function regionAt(x: number, y: number): DiagramRegion | undefined {
  return diagramRegions.find((region) => x >= region.x && x < region.x + region.width && y >= region.y && y < region.y + region.height);
}

export type InventoryCandidate = {
  id: string;
  name: string;
  management_endpoint: string;
  serial: string;
  site_id: string;
};

export type Suggestion = {
  deviceId: string;
  evidence: "management-address" | "serial" | "exact-name";
  ambiguous: boolean;
};

function normal(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
}

export function suggestDevices(label: string, devices: InventoryCandidate[]): Suggestion[] {
  const text = label.trim();
  if (!text) return [];
  const ip = text.match(/\b(?:\d{1,3}\.){3}\d{1,3}\b/)?.[0] || "";
  const folded = normal(text);
  const hits: Suggestion[] = [];
  for (const device of devices) {
    const endpoint = device.management_endpoint || "";
    if (ip && (endpoint === ip || endpoint.includes(ip))) hits.push({ deviceId: device.id, evidence: "management-address", ambiguous: false });
    else if (device.serial && device.serial.length > 3 && text.toLowerCase().includes(device.serial.toLowerCase())) hits.push({ deviceId: device.id, evidence: "serial", ambiguous: false });
    else if (folded.length > 2 && normal(device.name) === folded) hits.push({ deviceId: device.id, evidence: "exact-name", ambiguous: false });
  }
  const ambiguous = hits.length > 1;
  return hits.map((item) => ({ ...item, ambiguous }));
}
