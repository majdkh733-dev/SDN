# Deploy your first test connector

A site connector is a process inside the site network. It is not a vendor adapter, an integration, or a network controller. The API calls adapters. The browser does not open a session to a switch.

The connector uses the default Go HTTP client in `connectors/site-agent/loop.go`, so TLS certificates are validated. Do not set a custom transport that skips verification. The enrollment token is returned once, stored only as a hash, and expires in 15 minutes.

## Direction and ports

| From | To | Port | Direction |
| --- | --- | --- | --- |
| Connector | API (`SDN_API_URL`, the same origin as `PUBLIC_API_BASE`) | The API's HTTPS port | Outbound only |
| Connector | Site DNS resolver | 53 | Outbound, when reverse lookup is enabled |
| Connector | Addresses on the discovery profile allow-list | ICMP, TCP 22/80/443/161 by default, optional SNMP UDP/161 | Outbound only |

No inbound port is opened. These values come from `docs/connectors.md` and `networkRequirements` in `internal/httpserver/platform.go`.

## Lab simulator

In the console, Add connector, choose the site, and choose Lab simulator. That creates an adapter of kind `simulator`. No host package is installed. Inventory from it is labeled simulator and is not production health. If you need a process on an Ubuntu VM, use the site connector below instead. The simulator is not a fallback when that process fails.

## Ubuntu VM with Docker Compose

Build context is the repository root. The Docker method tags `sdn-connector:local` from `deploy/Dockerfile.connector`. Compose uses the same Dockerfile through `deploy/connector-compose.yml` and does not retag that name. Copy `deploy/connector.env.example` only as a reminder of the variable names. Compose reads `SDN_API_URL` and `SDN_ENROLLMENT_TOKEN` from the shell, not from that file.

1. Create the connector in the console with collection "Site connector" and method Compose. Copy the one-time token from the page. It is not shown again.
2. On the VM, from a checkout of this repository:

```sh
SDN_API_URL='https://the-api-origin' SDN_ENROLLMENT_TOKEN='paste-the-one-time-token' \
  docker compose -f deploy/connector-compose.yml up -d --build
```

`deploy/connector-compose.yml` sets `restart: unless-stopped`, stores the credential at `/var/lib/sdn-connector`, and uses the host network so ICMP and ARP can see the LAN. Health is the connector heartbeat to `POST /api/v1/agent/heartbeat`, not a separate container healthcheck.

3. Return to the connector page. Status becomes online after a heartbeat. A heartbeat shows the process can reach the API. It is not a completed discovery collection.
4. Create a discovery profile for an allow-listed prefix no shorter than /24, at most 256 addresses, and run discovery. Import observations from the job. Inventory and interfaces update only from returned observations.
5. Bind a discovered device to a diagram symbol on the Dashboard or Topology page. The drawing does not rearrange.

Uninstall:

```sh
docker compose -f deploy/connector-compose.yml down -v
```

Revoke the connector in the console as well. Revocation disables the credential. Imported assets stay.

## Linux without Docker

The console's Linux method prints the same steps as `installInstructions`: build `go build -o sdn-connector ./connectors/site-agent`, install it to `/usr/local/bin/sdn-connector`, write `/etc/sdn/connector.env`, and install the systemd unit embedded in `internal/httpserver/platform.go`. That unit uses `Restart=on-failure` and `EnvironmentFile=/etc/sdn/connector.env`.

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now sdn-connector
```

Uninstall is the command returned by `GET /api/v1/connectors/{id}/install?method=linux`.

## If a step fails

| What you see | What to check |
| --- | --- |
| Token rejected | It expired after 15 minutes, was already used, or belongs to another connector. Generate another token. |
| Stays connecting or offline | The process cannot reach `SDN_API_URL`. No inbound hole will fix an outbound TLS failure. |
| Heartbeat online, discovery empty | The allow-list does not include the device, or the probe ports are closed. A silent address is recorded as not reachable. |
| Inventory did not change | The job finished but observations were not imported. Import is a separate action. |

Helm, AWS, and Windows are not implemented. Do not invent an image name for them.
