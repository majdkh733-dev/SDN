import { classifySymbol, type SymbolKind } from "./classify";
import { plainLabel } from "./regions";

export type ViewBox = { x: number; y: number; w: number; h: number };

export type CellInfo = {
  id: string;
  kind: "vertex" | "edge";
  label: string;
  style: string;
  sourceId: string;
  targetId: string;
  sourceLabel: string;
  targetLabel: string;
  x: number;
  y: number;
  width: number;
  height: number;
};

export type DiagramHandle = {
  zoomBy: (factor: number) => void;
  fit: () => void;
  reset: () => void;
  focus: (rect: { x: number; y: number; width: number; height: number }) => void;
  find: (query: string) => string;
  select: (id: string) => void;
  info: (id: string) => CellInfo | null;
  setEditing: (on: boolean) => void;
  undo: () => void;
  redo: () => void;
  serialize: () => string;
  placeDevice: (label: string, style: string, width: number, height: number, dx?: number, dy?: number) => string;
  setLabel: (id: string, label: string) => void;
  symbols: () => (CellInfo & { symbolKind: SymbolKind })[];
};

type MxWindow = Window & {
  DRAWIO_BASE_URL?: string;
  SHAPES_PATH?: string;
  STENCIL_PATH?: string;
  STYLE_PATH?: string;
  GRAPH_IMAGE_PATH?: string;
  mxImageBasePath?: string;
  GraphViewer?: new (el: HTMLElement, node: Element, config: Record<string, unknown>) => { graph: MxGraph; editor: { undoManager?: { undo: () => void; redo: () => void } } };
  mxUtils?: { parseXml: (xml: string) => XMLDocument; getXml: (node: Node) => string };
  mxCodec?: new (doc?: XMLDocument) => { encode: (model: unknown) => Element };
  mxEvent?: { CLICK: string; UNDO: string };
  mxUndoManager?: new () => { undo: () => void; redo: () => void; undoableEditHappened: (edit: unknown) => void };
};

type MxCell = {
  id?: string;
  vertex?: boolean;
  edge?: boolean;
  value?: string;
  style?: string;
  getId: () => string;
  getValue: () => string;
  getStyle: () => string;
  isVertex: () => boolean;
  isEdge: () => boolean;
  getGeometry: () => { x: number; y: number; width: number; height: number } | null;
  getTerminal: (source: boolean) => MxCell | null;
};

type MxGraph = {
  getModel: () => {
    beginUpdate: () => void;
    endUpdate: () => void;
    getCell: (id: string) => MxCell | null;
    setValue: (cell: MxCell, value: string) => void;
    addListener: (name: string, fn: (sender: unknown, evt: { getProperty: (name: string) => unknown }) => void) => void;
  };
  getView: () => { addListener: (name: string, fn: (sender: unknown, evt: { getProperty: (name: string) => unknown }) => void) => void };
  getDefaultParent: () => MxCell;
  insertVertex: (parent: MxCell, id: string | null, value: string, x: number, y: number, w: number, h: number, style: string) => MxCell;
  setEnabled: (on: boolean) => void;
  setCellsMovable: (on: boolean) => void;
  setCellsResizable: (on: boolean) => void;
  setConnectable: (on: boolean) => void;
  setCellsEditable: (on: boolean) => void;
  setCellsBendable: (on: boolean) => void;
  setCellsDeletable: (on: boolean) => void;
  setDisconnectOnMove: (on: boolean) => void;
  setPanning: (on: boolean) => void;
  setSelectionCell: (cell: MxCell | null) => void;
  getSelectionCell: () => MxCell | null;
  getGraphBounds: () => ViewBox & { width: number; height: number };
  addListener: (name: string, fn: (sender: unknown, evt: { getProperty: (name: string) => unknown }) => void) => void;
  addMouseListener: (listener: {
    mouseDown: (sender: unknown, me: { state: unknown; getEvent: () => MouseEvent }) => void;
    mouseMove: (sender: unknown, me: { getEvent: () => MouseEvent }) => void;
    mouseUp: () => void;
  }) => void;
  sizeDidChange: () => void;
  container: HTMLElement;
};

let scriptPromise: Promise<void> | null = null;

export function loadDrawio() {
  if (scriptPromise) return scriptPromise;
  const w = window as MxWindow;
  w.DRAWIO_BASE_URL = "/drawio";
  w.SHAPES_PATH = "/drawio/shapes";
  w.STENCIL_PATH = "/drawio/stencils";
  w.STYLE_PATH = "/drawio/styles";
  w.GRAPH_IMAGE_PATH = "/drawio/img";
  w.mxImageBasePath = "/drawio/mxgraph/images";
  scriptPromise = new Promise((resolve, reject) => {
    if (w.GraphViewer && w.mxUtils) {
      resolve();
      return;
    }
    const script = document.createElement("script");
    script.src = "/drawio/viewer-static.min.js";
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("The diagram viewer did not load"));
    document.head.appendChild(script);
  });
  return scriptPromise;
}

function describe(cell: MxCell | null): CellInfo | null {
  if (!cell) return null;
  const source = cell.isEdge() ? cell.getTerminal(true) : null;
  const target = cell.isEdge() ? cell.getTerminal(false) : null;
  const geo = cell.getGeometry();
  return {
    id: cell.getId(),
    kind: cell.isEdge() ? "edge" : "vertex",
    label: plainLabel(String(cell.getValue() || "")),
    style: cell.getStyle() || "",
    sourceId: source ? source.getId() : "",
    targetId: target ? target.getId() : "",
    sourceLabel: source ? plainLabel(String(source.getValue() || "")) : "",
    targetLabel: target ? plainLabel(String(target.getValue() || "")) : "",
    x: geo?.x || 0,
    y: geo?.y || 0,
    width: geo?.width || 0,
    height: geo?.height || 0,
  };
}

export async function mountDiagram(container: HTMLElement, xml: string, onSelect: (id: string) => void): Promise<DiagramHandle> {
  await loadDrawio();
  const w = window as MxWindow;
  if (!w.GraphViewer || !w.mxUtils || !w.mxCodec || !w.mxEvent || !w.mxUndoManager) {
    throw new Error("The diagram viewer is unavailable");
  }
  container.replaceChildren();
  const doc = w.mxUtils.parseXml(xml);
  const viewer = new w.GraphViewer(container, doc.documentElement, {
    nav: false,
    resize: false,
    border: 0,
    lightbox: 0,
    "dark-mode": false,
    "check-visible-state": false,
    responsive: true,
    "auto-fit": true,
    "allow-zoom-out": true,
    "allow-zoom-in": true,
  });
  const graph = viewer.graph;
  const svg = container.querySelector("svg");
  if (!svg) throw new Error("The diagram did not render");
  const initial = svg.viewBox.baseVal;
  const full: ViewBox = { x: initial.x, y: initial.y, w: initial.width, h: initial.height };
  let view: ViewBox = { ...full };
  let editing = false;
  const apply = () => {
    svg.setAttribute("viewBox", `${view.x} ${view.y} ${view.w} ${view.h}`);
    svg.setAttribute("preserveAspectRatio", "xMidYMid meet");
    svg.style.width = "100%";
    svg.style.height = "100%";
    svg.style.background = "#ffffff";
  };
  const previous = graph.sizeDidChange.bind(graph);
  graph.sizeDidChange = () => {
    previous();
    apply();
  };
  apply();
  graph.setEnabled(true);
  const applyEdit = () => {
    graph.setCellsMovable(editing);
    graph.setCellsResizable(editing);
    graph.setConnectable(editing);
    graph.setCellsEditable(editing);
    graph.setCellsBendable(editing);
    graph.setCellsDeletable(editing);
    graph.setDisconnectOnMove(false);
    graph.setPanning(false);
  };
  applyEdit();
  const undo = new w.mxUndoManager();
  const hear = (_sender: unknown, evt: { getProperty: (name: string) => unknown }) => {
    const edit = evt.getProperty("edit");
    if (edit) undo.undoableEditHappened(edit);
  };
  graph.getModel().addListener(w.mxEvent.UNDO, hear);
  graph.getView().addListener(w.mxEvent.UNDO, hear);
  const hit = (event: MouseEvent): MxCell | null => {
    const rect = svg.getBoundingClientRect();
    const box = svg.viewBox.baseVal;
    if (rect.width < 1 || box.width < 1) return null;
    const fit = Math.min(rect.width / box.width, rect.height / box.height);
    const x = box.x + (event.clientX - rect.left - (rect.width - box.width * fit) / 2) / fit;
    const y = box.y + (event.clientY - rect.top - (rect.height - box.height * fit) / 2) / fit;
    const model = graph.getModel() as unknown as { getChildCount: (cell: MxCell | null) => number; getChildAt: (cell: MxCell | null, index: number) => MxCell; getRoot: () => MxCell };
    type State = { x: number; y: number; width: number; height: number; absolutePoints?: { x: number; y: number }[] | null };
    const states = graph.getView() as unknown as { getState: (cell: MxCell) => State | null };
    const tolerance = 10 / fit;
    let vertex: MxCell | null = null;
    let vertexArea = Number.POSITIVE_INFINITY;
    let edge: MxCell | null = null;
    let edgeDistance = tolerance;
    const nearSegment = (px: number, py: number, a: { x: number; y: number }, b: { x: number; y: number }) => {
      const dx = b.x - a.x;
      const dy = b.y - a.y;
      const len = dx * dx + dy * dy;
      const t = len === 0 ? 0 : Math.max(0, Math.min(1, ((px - a.x) * dx + (py - a.y) * dy) / len));
      const sx = a.x + t * dx - px;
      const sy = a.y + t * dy - py;
      return Math.hypot(sx, sy);
    };
    const visit = (parent: MxCell | null) => {
      const count = model.getChildCount(parent);
      for (let index = 0; index < count; index += 1) {
        const cell = model.getChildAt(parent, index);
        const id = cell.getId();
        if (id !== "0" && id !== "1") {
          const state = states.getState(cell);
          if (state && cell.isVertex() && state.width > 0 && state.height > 0 && x >= state.x && x <= state.x + state.width && y >= state.y && y <= state.y + state.height) {
            const area = state.width * state.height;
            if (area < vertexArea) {
              vertex = cell;
              vertexArea = area;
            }
          } else if (state && cell.isEdge()) {
            const points = state.absolutePoints || [];
            let distance = Number.POSITIVE_INFINITY;
            for (let point = 1; point < points.length; point += 1) distance = Math.min(distance, nearSegment(x, y, points[point - 1], points[point]));
            if (points.length < 2 && state.width >= 0 && state.height >= 0) {
              const cx = Math.max(state.x, Math.min(x, state.x + state.width));
              const cy = Math.max(state.y, Math.min(y, state.y + state.height));
              distance = Math.hypot(x - cx, y - cy);
            }
            if (distance <= edgeDistance) {
              edge = cell;
              edgeDistance = distance;
            }
          }
        }
        visit(cell);
      }
    };
    visit(model.getRoot());
    return vertex || edge;
  };
  container.addEventListener("click", (event) => {
    const cell = hit(event);
    if (!cell) return;
    graph.setSelectionCell(cell);
    onSelect(cell.getId());
  }, true);
  let pan: { x: number; y: number } | null = null;
  graph.addMouseListener({
    mouseDown: (_sender, me) => {
      if (editing || me.state) return;
      const event = me.getEvent();
      pan = { x: event.clientX, y: event.clientY };
    },
    mouseMove: (_sender, me) => {
      if (!pan) return;
      const event = me.getEvent();
      const rect = svg.getBoundingClientRect();
      if (rect.width < 1 || rect.height < 1) return;
      view.x -= ((event.clientX - pan.x) * view.w) / rect.width;
      view.y -= ((event.clientY - pan.y) * view.h) / rect.height;
      pan = { x: event.clientX, y: event.clientY };
      apply();
    },
    mouseUp: () => {
      pan = null;
    },
  });
  const cells = () => {
    const model = graph.getModel() as unknown as { cells?: Record<string, MxCell>; getDescendants?: (parent: MxCell) => MxCell[] };
    if (model.getDescendants) return model.getDescendants(graph.getDefaultParent());
    return Object.values(model.cells || {});
  };
  return {
    zoomBy: (factor) => {
      const w0 = view.w / factor;
      const h0 = view.h / factor;
      view = { x: view.x + (view.w - w0) / 2, y: view.y + (view.h - h0) / 2, w: w0, h: h0 };
      apply();
    },
    fit: () => {
      const bounds = graph.getGraphBounds();
      view = { x: bounds.x, y: bounds.y, w: Math.max(bounds.width, 1), h: Math.max(bounds.height, 1) };
      apply();
    },
    reset: () => {
      view = { ...full };
      apply();
    },
    focus: (rect) => {
      view = { x: rect.x - 30, y: rect.y - 30, w: rect.width + 60, h: rect.height + 60 };
      apply();
    },
    find: (query) => {
      const needle = query.trim().toLowerCase();
      if (!needle) return "";
      for (const cell of cells()) {
        if (!cell.getValue) continue;
        const label = plainLabel(String(cell.getValue() || "")).toLowerCase();
        if (label.includes(needle)) {
          graph.setSelectionCell(cell);
          const geo = cell.getGeometry();
          if (geo && geo.width) {
            view = { x: geo.x - 80, y: geo.y - 80, w: Math.max(geo.width + 160, 400), h: Math.max(geo.height + 160, 300) };
            apply();
          }
          return cell.getId();
        }
      }
      return "";
    },
    select: (id) => {
      const cell = graph.getModel().getCell(id);
      if (cell) graph.setSelectionCell(cell);
    },
    info: (id) => describe(graph.getModel().getCell(id)),
    setEditing: (on) => {
      editing = on;
      applyEdit();
    },
    undo: () => undo.undo(),
    redo: () => undo.redo(),
    serialize: () => {
      const encoded = new w.mxCodec!().encode(graph.getModel());
      const next = w.mxUtils!.parseXml(xml);
      const current = next.getElementsByTagName("mxGraphModel")[0];
      if (current?.parentNode) current.parentNode.replaceChild(next.importNode(encoded, true), current);
      return w.mxUtils!.getXml(next.documentElement);
    },
    placeDevice: (label, style, width, height, dx = 0, dy = 0) => {
      const parent = graph.getDefaultParent();
      graph.getModel().beginUpdate();
      let cell: MxCell;
      try {
        cell = graph.insertVertex(parent, null, label, view.x + view.w / 2 + dx, view.y + view.h / 2 + dy, width, height, style);
      } finally {
        graph.getModel().endUpdate();
      }
      graph.setSelectionCell(cell);
      return cell.getId();
    },
    symbols: () => cells().flatMap((cell) => {
      if (!cell.getId || cell.getId() === "0" || cell.getId() === "1") return [];
      const info = describe(cell);
      if (!info || (info.kind === "vertex" && !cell.isVertex())) return [];
      if (!cell.isVertex() && !cell.isEdge()) return [];
      return [{ ...info, symbolKind: classifySymbol({ vertex: cell.isVertex(), style: info.style, label: info.label, width: info.width, height: info.height }) }];
    }),
    setLabel: (id, label) => {
      const cell = graph.getModel().getCell(id);
      if (!cell) return;
      graph.getModel().beginUpdate();
      try {
        graph.getModel().setValue(cell, label);
      } finally {
        graph.getModel().endUpdate();
      }
    },
  };
}
