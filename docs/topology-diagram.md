# UTEC reference topology

The Topology page renders the original draw.io model embedded in `UTEC Diagram09-03-26.drawio.png`. The model was URL-decoded from the PNG `mxfile` text chunk and stored at `apps/web/public/drawio/utec.drawio`. Rendering uses the diagrams.net viewer (`viewer-static.min.js`) with the Cisco, network, electrical, Azure, and iOS stencil files the model references. Positions, styles, labels, waypoints, and embedded images come from that file. The page does not run a force-directed layout.

Icon fills such as green, amber, and red are the reference artwork. Operational health is shown only in the detail drawer.

## Sites

The selector zooms to the existing thick-border regions. It does not rearrange them.

| Region | How the name is known |
| --- | --- |
| UTEC | Logo image. The diagram title text is `UTEC IT Network Diagram`. |
| USSG | Logo image. OCR read `USSG`. |
| Epoxy | Logo image. OCR read `Epoxy`. A text label `Epoxy-RF WAN` also exists. |
| LV | Logo image. OCR read `LV`. |
| pakistan Office | Text cell, copied exactly, on a blue badge. |
| SS | Logo image in the right-hand band. The letters are artwork; OCR did not recover them. |
| Closed Heat Warehouse | Logo image. OCR read `Closed Heat Warehouse`. |
| Al Kharj Warehouse | Logo image in the lower-right band. The stylized wordmark was not recovered by OCR. |

## Inventory

Nothing in the reference file is bound to an inventory device until an engineer uses **Link to inventory device**. The diagram label stays as drawn unless **Use inventory name on the label** is used. A missing or deleted inventory record leaves the symbol in place and shows that the link is gone.

Devices that are not linked stay in **Unplaced devices**. Placing one is an explicit edit. It inserts a reference icon at the current view and saves a diagram version. Discovery does not insert devices into the drawing.

Links keep the documented path from the draw.io file. A link is marked verified only when both ends are linked to inventory and the existing topology API has a connection between those devices. Utilization is shown only when that API marks the value as measured.

## Editing

Engineers and administrators can move, resize, relabel, connect, undo, and redo. Viewers cannot. Save appends a version in `topology_diagrams` for the current organization. Bindings are stored separately in `topology_symbol_bindings`. Deleting a symbol does not delete an inventory device.

Reset view returns to the loaded diagram’s full bounds. Cancel edit reloads the last saved version, or the reference file when nothing has been saved.

## Templates

`apps/web/src/diagram/catalog.ts` copies device, boundary, separator, group, badge, and link styles from the source. New symbols must use one of those styles.

## What is still limited

Endpoint port names are not in the reference geometry. Neighbor tables, interface counters, and flow animation are not invented. Two logo wordmarks (SS and Al Kharj Warehouse) are visible as images; their selector names follow the artwork placement and were not a clean OCR read.

Most labels use Verdana, and the title uses Tahoma. Those faces are not bundled. Architects Daughter is bundled under its SIL Open Font License because the source references it. On a machine without Verdana or Tahoma, the same coordinates are kept and the fallback face changes only the glyph shapes.
