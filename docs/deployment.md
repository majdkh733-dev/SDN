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

To exercise the outbound connector, enroll a token in the console, then:

```sh
SDN_API_URL=http://127.0.0.1:8080 SDN_AGENT_TOKEN='the-token' SDN_SITE_NAME='Riyadh DC1' go run ./connectors/site-agent
```

Queue discovery with "Queue for site agent". With `INLINE_WORKFLOWS=false`, run `go run ./services/worker/cmd/worker` so approved changes are claimed outside the API process. The default local API runs workflows inline so the console can follow them without a second process.

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

Terminate TLS at the platform ingress. Set `ALLOWED_ORIGINS` to the Vercel origin and `PUBLIC_API_BASE` to the public API origin. Take database backups with the PostgreSQL host's backup tool. Application configuration backups are manifests inside the database; they are not a substitute for a database backup.

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

This environment does not have a Vercel token, so no deployment URL was created. Production launch remains incomplete until that project is created and the API, identity provider, database, and connectors are verified.

## Demonstration versus production

`AUTH_MODE=dev` and the simulator are a demonstration release. Production startup refuses that mode. Mark launch complete only after an external OIDC issuer, PostgreSQL, the API, the worker, and at least one enrolled connector have been exercised on the target network.
