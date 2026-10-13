# Deployment

## Local

PostgreSQL 16 is the only required service for local development. Docker is optional and is described by `deploy/docker-compose.yml`.

```sh
createdb sdn || true
cp .env.example .env
# Set TOKEN_SECRET, OIDC_CLIENT_SECRET, and DEV_ADMIN_PASSWORD in .env
set -a && . ./.env && set +a
go run ./services/api/cmd/api seed
go run ./services/api/cmd/api
```

In another shell:

```sh
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

Open http://localhost:5173. The Vite server proxies `/api` and `/oidc` to http://127.0.0.1:8080, so leave `VITE_API_BASE_URL` empty. Sign in with a seeded email and the `DEV_ADMIN_PASSWORD` you chose. Accounts:

- engineer@nexora.demo
- approver@nexora.demo
- admin@nexora.demo
- recovery@nexora.demo
- viewer@nexora.demo
- admin@isolated.demo (a second organization used to prove isolation)

Operations behavior, the adapter matrix, and the VLAN workflow are in `docs/platform.md`. Site connectors use a one-time enrollment token, then a stored credential. Install, uninstall, discovery, and the outbound port list are in `docs/connectors.md`. A short local run:

```sh
SDN_API_URL=http://127.0.0.1:8080 SDN_ENROLLMENT_TOKEN='paste-the-one-time-token' SDN_AGENT_STATE=/tmp/sdn-connector go run ./connectors/site-agent
```

Docker and systemd examples are in that document and in `deploy/connector-compose.yml`. Scheduled discovery needs the worker even when change workflows run inside the API:

```sh
go run ./services/worker/cmd/worker
```

With `INLINE_WORKFLOWS=false`, the same worker also claims confirmed changes. The default local API runs change workflows inline so the console can follow them without a second process. Leave `TEMPORAL_ADDRESS` empty unless a Temporal frontend is already running. An empty address keeps the PostgreSQL worker. `SECRET_BACKEND=memory` is the local default and stores nothing durable.

## Containers

```sh
export TOKEN_SECRET='...' OIDC_CLIENT_SECRET='...'
docker compose -f deploy/docker-compose.yml up --build
```

The API applies migrations on startup. Seed is a separate command:

```sh
docker compose -f deploy/docker-compose.yml run --rm api seed
```

Set `DEV_ADMIN_PASSWORD` for that command. Health checks are `GET /healthz` and `GET /readyz`. `GET /metrics` exposes `sdn_up 1`.

Terminate TLS at the platform ingress. Set `ALLOWED_ORIGINS` to the Vercel origin and `PUBLIC_API_BASE` to the public API origin. The API also allows `https://sdn-psi.vercel.app` and this project's preview hosts (`https://sdn-git-*-majd-6c6d.vercel.app`). Take database backups with the PostgreSQL host's backup tool. Application configuration backups are manifests inside the database; they are not a substitute for a database backup.

## Oracle and Vercel

`https://sdn-psi.vercel.app` is the Vercel production build of `main`. That build still draws the Command Room with the sites, network, and geographic lenses. It does not contain `utec.drawio` or `GET /api/v1/topology-diagram`. The recovered diagram and the enterprise sidebar are on `cursor/command-room-utec-9880`. Publishing the frontend means deploying that commit from `apps/web` with `VITE_API_BASE_URL=https://sdn-api.84-8-104-48.sslip.io`.

The Oracle API must be the same commit. Startup applies only migration files missing from `schema_migrations`. This upgrade adds `006_operations.sql`, `007_topology_diagram.sql`, and `008_topology_links.sql` when they are not already recorded. Do not run `seed`.

```sh
cd /opt/sdn
git fetch origin cursor/command-room-utec-9880
git checkout cursor/command-room-utec-9880
CGO_ENABLED=0 go build -o /usr/local/bin/sdn-api ./services/api/cmd/api
sudo systemctl restart sdn-api
curl -fsS https://sdn-api.84-8-104-48.sslip.io/readyz
curl -sS -o /dev/null -w '%{http_code}\n' https://sdn-api.84-8-104-48.sslip.io/api/v1/topology-diagram
```

An unauthenticated diagram request should be JSON `401`, not `404 page not found`. Keep the existing environment file and database. If the unit name differs, restart the unit from `systemctl list-units --type=service | grep -i sdn`. For Docker, `docker compose -f deploy/docker-compose.yml up -d --build api` and do not run the seed service. Caddy already forwards these paths.

The site agent runs inside the managed network and dials out. Do not publish device management ports to the API host.

## Vercel

| Setting | Value |
| --- | --- |
| Repository | majdkh733-dev/SDN |
| Root directory | apps/web |
| Framework | Vite |
| Build command | npm run build |
| Output directory | dist |
| Production branch | main |

`apps/web/vercel.json` rewrites unknown paths to `index.html` so deep links survive refresh.

Set `VITE_API_BASE_URL` to the public API origin before the production build. Because that origin differs from the Vercel origin, set the API to `COOKIE_SAMESITE=none` and `SECURE_COOKIES=true`, and include the Vercel origin in `ALLOWED_ORIGINS` and the callback in `OIDC_REDIRECT_URLS`.

Production launch remains incomplete until the Vercel project exists and the API, external OIDC issuer, PostgreSQL, Vault, and connectors are verified. The API process also has to be reachable at `VITE_API_BASE_URL`. A missing `VERCEL_TOKEN` means this repository cannot create that project.

## Demonstration versus production

`AUTH_MODE=dev` and the simulator are a demonstration release. Production startup refuses that mode. Mark launch complete only after an external OIDC issuer, PostgreSQL, the API, the worker, and at least one enrolled connector have been exercised on the target network.
