# ADR 001 — Go modular monolith and a separate worker

The API is one Go process with packages for configuration, identity, domain rules, persistence, and HTTP. A second process, `services/worker`, claims approved changes and runs the same store methods. The site agent is a third process and only speaks HTTP to the API.

The external contract is REST, documented in `packages/contracts/openapi.yaml`. gRPC is not introduced. There is one client, the console, and the connector protocol is a small set of task endpoints.

The browser is a separate Vite application so Vercel can publish it without the database or the worker.
