// Styles copied from the UTEC draw.io source. Icon colors are reference artwork, not live health.
export type DiagramStyle = { id: string; label: string; kind: string; style: string; width?: number; height?: number };
export const diagramCatalog: DiagramStyle[] = [
  {
    "id": "access-switch",
    "label": "Access switch",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "points=[[0.015,0.015,0],[0.985,0.015,0],[0.985,0.985,0],[0.015,0.985,0],[0.25,0,0],[0.5,0,0],[0.75,0,0],[1,0.25,0],[1,0.5,0],[1,0.75,0],[0.75,1,0],[0.5,1,0],[0.25,1,0],[0,0.75,0],[0,0.5,0],[0,0.25,0]];verticalLabelPosition=bottom;html=1;verticalAlign=top;aspect=fixed;align=center;pointerEvents=1;shape=mxgraph.cisco19.rect;prIcon=l2_switch;fillColor=none;strokeColor=#b85450;hachureGap=4;fontFamily=Architects Daughter;fontSource=https%3A%2F%2Ffonts.googleapis.com%2Fcss%3Ffamily%3DArchitects%2BDaughter;rounded=0;"
  },
  {
    "id": "access-switch-amber",
    "label": "Access switch amber",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "points=[[0.015,0.015,0],[0.985,0.015,0],[0.985,0.985,0],[0.015,0.985,0],[0.25,0,0],[0.5,0,0],[0.75,0,0],[1,0.25,0],[1,0.5,0],[1,0.75,0],[0.75,1,0],[0.5,1,0],[0.25,1,0],[0,0.75,0],[0,0.5,0],[0,0.25,0]];verticalLabelPosition=bottom;html=1;verticalAlign=top;aspect=fixed;align=center;pointerEvents=1;shape=mxgraph.cisco19.rect;prIcon=l2_switch;fillColor=#FFF2CC;strokeColor=#d6b656;hachureGap=4;fontFamily=Architects Daughter;fontSource=https%3A%2F%2Ffonts.googleapis.com%2Fcss%3Ffamily%3DArchitects%2BDaughter;rounded=0;"
  },
  {
    "id": "access-switch-red",
    "label": "Access switch red",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "points=[[0.015,0.015,0],[0.985,0.015,0],[0.985,0.985,0],[0.015,0.985,0],[0.25,0,0],[0.5,0,0],[0.75,0,0],[1,0.25,0],[1,0.5,0],[1,0.75,0],[0.75,1,0],[0.5,1,0],[0.25,1,0],[0,0.75,0],[0,0.5,0],[0,0.25,0]];verticalLabelPosition=bottom;html=1;verticalAlign=top;aspect=fixed;align=center;pointerEvents=1;shape=mxgraph.cisco19.rect;prIcon=l2_switch;fillColor=#F8CECC;strokeColor=#b85450;hachureGap=4;fontFamily=Architects Daughter;fontSource=https%3A%2F%2Ffonts.googleapis.com%2Fcss%3Ffamily%3DArchitects%2BDaughter;gradientColor=none;rounded=0;"
  },
  {
    "id": "distribution-switch-green",
    "label": "Distribution switch green",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "points=[[0.015,0.015,0],[0.985,0.015,0],[0.985,0.985,0],[0.015,0.985,0],[0.25,0,0],[0.5,0,0],[0.75,0,0],[1,0.25,0],[1,0.5,0],[1,0.75,0],[0.75,1,0],[0.5,1,0],[0.25,1,0],[0,0.75,0],[0,0.5,0],[0,0.25,0]];verticalLabelPosition=bottom;html=1;verticalAlign=top;aspect=fixed;align=center;pointerEvents=1;shape=mxgraph.cisco19.rect;prIcon=l3_switch;fillColor=#D5E8D4;strokeColor=#82b366;hachureGap=4;fontFamily=Architects Daughter;fontSource=https%3A%2F%2Ffonts.googleapis.com%2Fcss%3Ffamily%3DArchitects%2BDaughter;rounded=0;"
  },
  {
    "id": "server",
    "label": "Server",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "verticalAlign=top;verticalLabelPosition=bottom;labelPosition=center;align=center;html=1;outlineConnect=0;fillColor=#f8cecc;strokeColor=#b85450;gradientDirection=north;strokeWidth=2;shape=mxgraph.networks.server;hachureGap=4;fontFamily=Architects Daughter;fontSource=https%3A%2F%2Ffonts.googleapis.com%2Fcss%3Ffamily%3DArchitects%2BDaughter;shadow=1;rounded=0;"
  },
  {
    "id": "access-switch-green",
    "label": "Access switch green",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "points=[[0.015,0.015,0],[0.985,0.015,0],[0.985,0.985,0],[0.015,0.985,0],[0.25,0,0],[0.5,0,0],[0.75,0,0],[1,0.25,0],[1,0.5,0],[1,0.75,0],[0.75,1,0],[0.5,1,0],[0.25,1,0],[0,0.75,0],[0,0.5,0],[0,0.25,0]];verticalLabelPosition=bottom;html=1;verticalAlign=top;aspect=fixed;align=center;pointerEvents=1;shape=mxgraph.cisco19.rect;prIcon=l2_switch;fillColor=#D5E8D4;strokeColor=#82b366;hachureGap=4;fontFamily=Architects Daughter;fontSource=https%3A%2F%2Ffonts.googleapis.com%2Fcss%3Ffamily%3DArchitects%2BDaughter;rounded=0;"
  },
  {
    "id": "distribution-switch",
    "label": "Distribution switch",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "points=[[0.015,0.015,0],[0.985,0.015,0],[0.985,0.985,0],[0.015,0.985,0],[0.25,0,0],[0.5,0,0],[0.75,0,0],[1,0.25,0],[1,0.5,0],[1,0.75,0],[0.75,1,0],[0.5,1,0],[0.25,1,0],[0,0.75,0],[0,0.5,0],[0,0.25,0]];verticalLabelPosition=bottom;html=1;verticalAlign=top;aspect=fixed;align=center;pointerEvents=1;shape=mxgraph.cisco19.rect;prIcon=l3_switch;fillColor=none;strokeColor=#82b366;hachureGap=4;fontFamily=Architects Daughter;fontSource=https%3A%2F%2Ffonts.googleapis.com%2Fcss%3Ffamily%3DArchitects%2BDaughter;rounded=0;"
  },
  {
    "id": "router",
    "label": "Router",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "shape=mxgraph.cisco.routers.router;html=1;pointerEvents=1;dashed=0;fillColor=#dae8fc;strokeColor=#6c8ebf;strokeWidth=2;verticalLabelPosition=bottom;verticalAlign=top;align=center;outlineConnect=0;shadow=1;gradientColor=#7ea6e0;rounded=0;"
  },
  {
    "id": "content-router",
    "label": "Content router",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "shape=mxgraph.cisco.routers.content_service_router;html=1;pointerEvents=1;dashed=0;fillColor=#dae8fc;strokeColor=#6c8ebf;strokeWidth=2;verticalLabelPosition=bottom;verticalAlign=top;align=center;outlineConnect=0;shadow=1;gradientColor=#7ea6e0;rounded=0;"
  },
  {
    "id": "wireless-backhaul",
    "label": "Wireless backhaul",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "shape=mxgraph.cisco.wireless.satellite_dish;html=1;pointerEvents=1;dashed=0;fillColor=#d5e8d4;strokeColor=#82b366;strokeWidth=2;verticalLabelPosition=bottom;verticalAlign=top;align=center;outlineConnect=0;rotation=-90;gradientColor=#97d077;rounded=0;"
  },
  {
    "id": "storage",
    "label": "Storage",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "verticalAlign=top;verticalLabelPosition=bottom;labelPosition=center;align=center;html=1;outlineConnect=0;fillColor=#d5e8d4;strokeColor=#82b366;gradientDirection=north;strokeWidth=2;shape=mxgraph.networks.nas_filer;gradientColor=#97d077;rounded=0;"
  },
  {
    "id": "firewall",
    "label": "Firewall",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "verticalAlign=top;verticalLabelPosition=bottom;labelPosition=center;align=center;outlineConnect=0;fillColor=#d5e8d4;strokeColor=#82b366;gradientDirection=north;strokeWidth=2;shape=mxgraph.networks.firewall;hachureGap=4;fontFamily=Helvetica;html=1;shadow=1;rounded=0;"
  },
  {
    "id": "wireless-access-point",
    "label": "Wireless access point",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "html=1;strokeWidth=1;shadow=0;dashed=0;shape=mxgraph.ios7.misc.wifi;fillColor=#d5e8d4;strokeColor=#82b366;buttonText=;strokeColor2=#222222;fontSize=8;verticalLabelPosition=bottom;verticalAlign=top;align=center;rounded=0;"
  },
  {
    "id": "fiber",
    "label": "Fiber",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "verticalLabelPosition=bottom;shadow=0;dashed=0;align=center;html=1;verticalAlign=top;shape=mxgraph.electrical.transmission.optical_fiber2;fillColor=none;strokeColor=#006EAF;fontColor=#ffffff;rounded=0;"
  },
  {
    "id": "wan-router",
    "label": "WAN router",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "shape=mxgraph.cisco.routers.gigabit_switch_atm_tag_router;html=1;pointerEvents=1;dashed=0;fillColor=#dae8fc;strokeColor=#6c8ebf;strokeWidth=2;verticalLabelPosition=bottom;verticalAlign=top;align=center;outlineConnect=0;shadow=1;gradientColor=#7ea6e0;rounded=0;"
  },
  {
    "id": "server-block",
    "label": "Server block",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "aspect=fixed;pointerEvents=1;shadow=1;dashed=0;html=1;strokeColor=#82b366;labelPosition=center;verticalLabelPosition=bottom;verticalAlign=top;align=center;fillColor=#d5e8d4;shape=mxgraph.azure.server;gradientColor=#97d077;rounded=0;"
  },
  {
    "id": "wavelength-router",
    "label": "Wavelength router",
    "kind": "device",
    "width": 100,
    "height": 100,
    "style": "shape=mxgraph.cisco.routers.wavelength_router;html=1;pointerEvents=1;dashed=0;fillColor=light-dark(#A5CBA4,#1F2F1E);strokeColor=#82B366;strokeWidth=2;verticalLabelPosition=bottom;verticalAlign=top;align=center;outlineConnect=0;gradientColor=none;strokeColor2=#ffffff;shadow=1;gradientDirection=south;"
  },
  {
    "id": "site-boundary",
    "label": "Site boundary",
    "kind": "boundary",
    "style": "line;strokeWidth=7;html=1;perimeter=backbonePerimeter;points=[];"
  },
  {
    "id": "layer-separator",
    "label": "Layer separator",
    "kind": "separator",
    "style": "line;strokeWidth=2;dashed=1;html=1;"
  },
  {
    "id": "building-group",
    "label": "Building group",
    "kind": "group",
    "style": "shape=partialRectangle;html=1;whiteSpace=wrap;connectable=0;fillColor=none;strokeColor=#000000;strokeWidth=2;"
  },
  {
    "id": "documented-link",
    "label": "Documented link",
    "kind": "link",
    "style": "edgeStyle=orthogonalEdgeStyle;rounded=0;orthogonalLoop=1;jettySize=auto;html=1;endArrow=none;endFill=0;"
  },
  {
    "id": "site-badge",
    "label": "Site badge",
    "kind": "label",
    "style": "text;html=1;strokeColor=none;fillColor=none;align=center;verticalAlign=middle;labelBackgroundColor=#0056AC;fontColor=#ffffff;fontSize=16;fontFamily=Verdana;"
  }
];
