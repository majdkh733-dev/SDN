import cytoscape, { type Core, type LayoutOptions } from "cytoscape";
import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";

export type TopoNode = {
  id: string;
  label: string;
  kind: string;
  health: string;
  site_id?: string;
  meta?: Record<string, unknown>;
};

export type TopoEdge = {
  id: string;
  source: string;
  target: string;
  utilization_pct: number;
  label: string;
};

export type GraphHandle = { zoomBy: (factor: number) => void; fit: () => void };

function token(name: string, fallback: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
}

export const TopologyGraph = forwardRef<GraphHandle, {
  nodes: TopoNode[];
  edges: TopoEdge[];
  selected: string;
  highlighted: string[];
  onSelect: (id: string) => void;
}>(function TopologyGraph({
  nodes,
  edges,
  selected,
  highlighted,
  onSelect,
}, ref) {
  const host = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const onSelectRef = useRef(onSelect);
  onSelectRef.current = onSelect;
  const signature = JSON.stringify({ nodes, edges, highlighted });
  useImperativeHandle(ref, () => ({
    zoomBy(factor: number) {
      const cy = cyRef.current;
      if (!cy) return;
      cy.zoom({ level: Math.min(2.5, Math.max(0.3, cy.zoom() * factor)), renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
    },
    fit() { cyRef.current?.fit(undefined, 24); },
  }));

  useEffect(() => {
    if (!host.current) return;
    const highlightedSet = new Set(highlighted);
    const cy = cytoscape({
      container: host.current,
      elements: [
        ...nodes.map((node) => ({
          data: {
            id: node.id,
            label: node.label,
            health: node.health,
            kind: node.kind,
            fill: node.health === "down" ? token("--danger", "#8a3d3d") : node.meta?.telemetry === "stale" ? token("--warn", "#8a6a2c") : token("--ok", "#2f7d4f"),
          },
          classes: highlightedSet.has(node.id) ? "highlight" : "",
        })),
        ...edges.map((edge) => ({
          data: { id: edge.id, source: edge.source, target: edge.target, label: edge.label, weight: edge.utilization_pct || 1 },
        })),
      ],
      style: [
        {
          selector: "node",
          style: {
            "background-color": "data(fill)",
            label: "data(label)",
            color: getComputedStyle(document.documentElement).getPropertyValue("--text").trim() || "#1c2834",
            "font-size": 11,
            "font-family": "Source Sans 3, IBM Plex Sans Arabic, sans-serif",
            "text-valign": "bottom",
            "text-margin-y": 6,
            width: 26,
            height: 26,
            "border-width": 1,
            "border-color": "#d3e0ec",
          },
        },
        { selector: "node[health = 'down']", style: { "background-color": "#8a3d3d" } },
        { selector: "node.highlight", style: { "border-width": 3, "border-color": "#8a6a2c" } },
        { selector: "node:selected", style: { "border-width": 3, "border-color": "#1c2834" } },
        {
          selector: "edge",
          style: {
            width: "mapData(weight, 0, 100, 1, 4)",
            "line-color": "#8aa0b5",
            "curve-style": "bezier",
            label: "data(label)",
            "font-size": 10,
            color: "#5e6c7b",
            "text-rotation": "autorotate",
          },
        },
      ],
      layout: {
        name: "cose",
        animate: !window.matchMedia("(prefers-reduced-motion: reduce)").matches,
        animationDuration: 240,
        fit: true,
        padding: 24,
      } as LayoutOptions,
      userZoomingEnabled: true,
      boxSelectionEnabled: false,
    });
    cy.on("tap", "node, edge", (event) => onSelectRef.current(String(event.target.id())));
    cyRef.current = cy;
    return () => {
      cy.destroy();
      cyRef.current = null;
    };
  }, [signature]);

  useEffect(() => {
    const cy = cyRef.current;
    if (!cy) return;
    cy.nodes().unselect();
    if (selected) cy.$id(selected).select();
  }, [selected, signature]);

  return <div ref={host} className="graph" role="img" aria-label="Network topology" />;
});
