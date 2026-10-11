# SDN

SDN is a multi-site network fabric and policy orchestration console. Operators discover inventory, preview policy impact, collect a separate approval, and execute only after the server rechecks the plan. The browser talks to the API. It does not connect to network devices.

This repository is a working demonstration against a deterministic simulator. The OVN adapter is implemented and has not been verified in a lab. Other southbound integrations are listed and are not implemented. Production launch is complete only after the API, external OIDC, PostgreSQL, connectors, and workflows are deployed and verified. See `docs/master-plan.md`.

## Layout

- `apps/web` — React, TypeScript, and Vite console, published from Vercel
- `services/api` — Go API
- `services/worker` — background change runner
- `connectors/site-agent` — outbound site connector
- `connectors/ovn` — OVN northbound adapter
- `packages/contracts/openapi.yaml` — HTTP contract
- `deploy` — container images and Compose
- `docs` — architecture, schema, security, and the checklist

## Local start

PostgreSQL 16 must be running. Copy `.env.example` to `.env` and set `TOKEN_SECRET`, `OIDC_CLIENT_SECRET`, and `DEV_ADMIN_PASSWORD`.

```sh
set -a && . ./.env && set +a
go run ./services/api/cmd/api seed
go run ./services/api/cmd/api
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

The console is at http://localhost:5173. Sign in with `engineer@nexora.demo` and the password you set. Approval uses `approver@nexora.demo`. Restore uses `recovery@nexora.demo`. Full deployment notes, including Vercel settings, are in `docs/deployment.md`.

## Tests

```sh
make test-go
npm --prefix apps/web ci
npm --prefix apps/web test
npm --prefix apps/web run build
```

`make test-go` skips `apps/web`, which is a Node project.
