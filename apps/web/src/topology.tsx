import cytoscape, { type Core, type LayoutOptions } from "cytoscape";
import { useEffect, useRef } from "react";

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

export function TopologyGraph({
  nodes,
  edges,
  selected,
  highlighted,
  onSelect,
}: {
  nodes: TopoNode[];
  edges: TopoEdge[];
  selected: string;
  highlighted: string[];
  onSelect: (id: string) => void;
}) {
  const host = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const onSelectRef = useRef(onSelect);
  onSelectRef.current = onSelect;
  const signature = JSON.stringify({ nodes, edges, highlighted });

  useEffect(() => {
    if (!host.current) return;
    const highlightedSet = new Set(highlighted);
    const cy = cytoscape({
      container: host.current,
      elements: [
        ...nodes.map((node) => ({
          data: { id: node.id, label: node.label, health: node.health, kind: node.kind },
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
            "background-color": "#2c5f8a",
            label: "data(label)",
            color: "#1c2834",
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
      layout: { name: "cose", animate: true, animationDuration: 240, fit: true, padding: 24 } as LayoutOptions,
      userZoomingEnabled: true,
      boxSelectionEnabled: false,
    });
    cy.on("tap", "node", (event) => onSelectRef.current(String(event.target.id())));
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
}
