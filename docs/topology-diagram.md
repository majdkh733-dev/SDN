# UTEC reference topology

The Dashboard and the Topology page render the same original draw.io model embedded in `UTEC Diagram09-03-26.drawio.png`. The model was URL-decoded from the PNG `mxfile` text chunk and stored at `apps/web/public/drawio/utec.drawio`. Rendering uses the diagrams.net viewer (`viewer-static.min.js`) with the Cisco, network, electrical, Azure, and iOS stencil files the model references. Positions, styles, labels, waypoints, and embedded images come from that file. Neither view runs a force-directed layout. Both views load `GET /api/v1/topology-diagram`, so they show the same saved version and the same bindings.

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

Nothing in the reference file is bound to an inventory device until an engineer uses **Link to inventory device**. Search is limited to the current organization's inventory, with an optional site filter. Suggestions appear only for an exact management address, an exact serial number contained in the label, or an exact normalized name. Several matches are marked for review and are not applied until the user confirms. Similar labels are not bound automatically, and the product does not create inventory records for unmapped symbols.

The diagram label stays as drawn unless **Use inventory name on the label** is used. A missing or deleted inventory record leaves the symbol in place and shows that the link is gone. Binding the same device to a second symbol returns `409 duplicate_binding`. Bindings are keyed by the draw.io cell id in `topology_symbol_bindings`.

The mapping table lists device symbols only. Labels, logos, groups, clouds, site borders, and dashed separators are classified separately and are not offered as device mappings. Device icons in the source usually have an empty cell value; the readable names are separate text cells, so the table shows the stencil name and the stable cell id instead of guessing a nearby caption.

Devices that are not linked stay in **Unplaced devices**. Placing one is an explicit edit. It inserts a reference icon at the current view and saves a diagram version. Discovery does not insert devices into the drawing or move existing symbols.

Reference lines keep the documented path from the draw.io file. An authorized user can map a line to a source device and interface and a destination device and interface, with a connection type (`documented`, `physical`, `logical`, `wan`) and evidence (`manual`, `discovered`, `documented`). Those rows live in `topology_link_bindings`. A line stays unresolved until both interfaces are saved. A manual mapping stays distinct from a connection returned by the topology API. Utilization is shown only when that API marks the value as measured. Interface error counters are not collected. Site borders and dashed separators are not device links.

Clicking a mapped symbol loads the inventory device: name, management address, site, vendor, model, serial, firmware, connector reachability, interfaces, neighbors from the topology API, site VLANs, policies whose scope includes the site, organization backup manifests, drift findings, and changes. Icon color stays the reference artwork. A device with no connector is reported as reachability unknown, not offline.

## Editing

Engineers and administrators can move, resize, relabel, connect, bend link waypoints, undo, and redo. Edit mode can also insert a group or a site frame with the reference layer labels. Viewers cannot save a diagram, bind a symbol, or map a link; the server returns 403. Save appends a version in `topology_diagrams` for the current organization. Restoring a version appends that mxfile again and leaves bindings in place for cell ids that still exist. Deleting a symbol does not delete an inventory device. Refreshing telemetry refetches device and topology data and does not reload the drawing.

Reset view returns to the loaded diagram’s full bounds. Cancel edit reloads the last saved version, or the reference file when nothing has been saved.

## Templates

`apps/web/src/diagram/catalog.ts` copies device, boundary, separator, group, badge, and link styles from the source. New symbols must use one of those styles.

## Migration

`007_topology_diagram.sql` creates diagram versions and symbol bindings. `008_topology_links.sql` creates link bindings. Both apply when the API starts. Diagram saves may be up to 12 MB of mxfile text; other request bodies stay at 1 MB. No production deploy is part of this change.

## What is still limited

The demo inventory does not contain the UTEC reference devices. A symbol stays unmapped until someone links a real inventory record. The seeded simulator switches can be linked manually; that does not mean the reference diagram is connected to the UTEC network.

Endpoint port names are not in the reference geometry. Interface error counters, routing tables, and device configuration text are not collected. Organization backups are manifests of counts, not running-config snapshots. Neighbor rows come only from topology links the API already has. A site connector is required for live discovery; the simulator connector covers only the lab switches. Two logo wordmarks (SS and Al Kharj Warehouse) are visible as images; their selector names follow the artwork placement and were not a clean OCR read.

Most labels use Verdana, and the title uses Tahoma. Those faces are not bundled. Architects Daughter is bundled under its SIL Open Font License because the source references it. On a machine without Verdana or Tahoma, the same coordinates are kept and the fallback face changes only the glyph shapes.
