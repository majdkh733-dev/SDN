# Site connectors and discovery

A connector is a process inside a site network. The browser and the cloud API do not scan that network. The connector opens an outbound HTTPS connection to `PUBLIC_API_BASE`, enrolls once, then polls for work.

## Network

Outbound only:

- HTTPS to the API origin (`SDN_API_URL` / `PUBLIC_API_BASE`), including its port.
- DNS to the site resolver for reverse lookups.
- ICMP echo, the profile's TCP ports (default 22, 80, 443, 161), and SNMP UDP/161 only toward addresses on that profile's allow-list.

No inbound port is required. These are not copied from another product's connector.

Scans accept at most 256 IPv4 addresses and reject prefixes shorter than /24. Exclusions are applied before probing. Discovery does not authenticate by guessing, and it does not change device configuration.

## Enrollment

1. An administrator creates a connector for one site and chooses Docker, Compose, or Linux.
2. The API returns a one-time enrollment token. It expires in 15 minutes, is stored as a hash, and is not written to logs.
3. The connector exchanges that token at `POST /api/v1/agent/enroll` and receives a credential once.
4. The credential is stored in `SDN_AGENT_STATE` with mode `0600` when that path is set.
5. Heartbeats update connector status only. An offline connector does not mark site devices down. Their telemetry becomes stale or unknown.

Revoke disables the credential. Replace revokes credentials and requires a new enrollment token. Remove archives the connector and leaves imported assets in place.

Helm, AWS, and Windows cards are labeled unavailable. They do not show install commands.

## Docker

```sh
docker build -f deploy/Dockerfile.connector -t sdn-connector:local .
docker run -d --name sdn-connector --restart unless-stopped --network host \
  -v sdn-connector:/var/lib/sdn-connector \
  -e SDN_API_URL='http://127.0.0.1:8080' \
  -e SDN_ENROLLMENT_TOKEN='paste-the-one-time-token' \
  -e SDN_AGENT_STATE=/var/lib/sdn-connector/credential \
  sdn-connector:local
```

`--network host` is what lets ARP and ICMP see the site LAN. A bridge network only sees the Docker network.

Compose:

```sh
SDN_API_URL='http://127.0.0.1:8080' SDN_ENROLLMENT_TOKEN='paste-the-one-time-token' \
  docker compose -f deploy/connector-compose.yml up -d --build
```

Uninstall:

```sh
docker rm -f sdn-connector
docker volume rm sdn-connector
```

Compose: `docker compose -f deploy/connector-compose.yml down -v`.

## Linux

```sh
go build -o sdn-connector ./connectors/site-agent
sudo install -d -m 0700 /var/lib/sdn-connector /etc/sdn
sudo install -m 0755 sdn-connector /usr/local/bin/sdn-connector
```

Write `/etc/sdn/connector.env` with `SDN_API_URL`, `SDN_ENROLLMENT_TOKEN`, and `SDN_AGENT_STATE=/var/lib/sdn-connector/credential`. Install the unit from the connector page, then:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now sdn-connector
```

Uninstall stops the unit, deletes the binary, the env file, and `/var/lib/sdn-connector`, then reloads systemd. Revoke the connector in the console as well.

The API process does not run the connector. `services/worker` enqueues scheduled scans. With `INLINE_WORKFLOWS=true` the API still serves jobs, and the connector polls either way. No Temporal cluster is required for discovery.

## What a scan can identify

| Signal | Recorded when |
| --- | --- |
| ICMP echo | `ping` succeeds |
| TCP connect | one of the profile ports accepts a connection |
| ARP | a complete neighbour entry exists on the connector host |
| DNS | a reverse name is returned |
| SNMP v2c | a profile stored a read-only community and `sysDescr` or `sysName` is returned |

An explicit address that stays silent is still listed as not reachable. That is not proof the address is unused. A host that fails ICMP can still be recorded from TCP, ARP, or SNMP. Vendor, model, serial, device type, and topology links stay empty unless a method actually returned them. Sharing a subnet does not create a link. Link latency is shown only when a site link has a measured value.

## Asset identity

Imports are scoped to the organization and the site, so overlapping private addresses at two sites are different assets.

1. Serial number, when present.
2. Otherwise MAC address.
3. Otherwise the IP address.

A later scan updates the same asset. Fields an operator locks, including a renamed asset, are kept. The same IP with a different MAC is marked ambiguous and is not merged until someone reviews it. CSV import uses the same identity rules. There was no CSV importer before this change; `POST /api/v1/sites/{id}/assets/import` is that path.

## Capabilities

Discovery records whether inventory, interfaces, topology, health monitoring, and policy changes are supported, with the evidence. A discovered address is not a manageable device. Policy changes stay on the existing approval workflow and only run through an adapter that implements them. The site connector's policy capability is unsupported until a management adapter is enrolled.

## Limits verified here

`TestConnectorDiscoveryLifecycle` enrolled a connector, rejected an expired and a reused token, isolated a second organization, imported one observation twice without a duplicate, kept a locked name, and rejected a revoked credential. `TestAgentEnrollAndDiscover` probed a local TCP listener from the connector loop. No physical switch, SNMP agent, or cloud host was available, so those paths were not exercised on a device.
