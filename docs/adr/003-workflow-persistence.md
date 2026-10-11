# ADR 003 — Durable workflows in PostgreSQL

PostgreSQL is the workflow driver exercised in this environment. `TEMPORAL_ADDRESS` was empty, Compose does not start a Temporal server, and no change here was executed by a Temporal cluster.

`internal/orchestration` imports `go.temporal.io/sdk` v1.31.0. `ExecuteChange` is a workflow whose activity calls `RunChange` with the original organization id and change id. `TestExecuteChangeUsesTheSuppliedIdentifiers` runs that workflow in the Temporal SDK test suite, in process, with no server. When `TEMPORAL_ADDRESS` is set and `INLINE_WORKFLOWS=false`, the API starts one workflow id `sdn-change-{id}` and the worker polls that task queue instead of claiming rows. A duplicate start is treated as the existing execution. Production refuses to combine inline execution with a Temporal address.

What the PostgreSQL driver does, and what `TestWorkflowRecovery` covers:

- Approval leaves `author_confirmed` false. `RunChange` returns forbidden and the worker claim skips the row.
- The author must confirm the plan hash. That flag is what makes an approved change claimable.
- `workflow_runs` and `execution_steps` persist each attempt. Steps are unique per change and operation key.
- `RunChange` takes a PostgreSQL advisory lock on the change id.
- Recovery of verifying, compensating, partial, applying, and unknown states uses the stored policy version and the stored device list. A newer version and a device added after approval do not change that plan.
- Completed and skipped rollback steps are not repeated. `compensation_failed` is preserved, the ACL stays, and the change remains `rollback_failed`.
- An uncertain device result stops the pass. The next run inspects `simulator_acls` before inserting again.
- Cancellation is a flag the runner observes between devices.

`INLINE_WORKFLOWS=true` runs the PostgreSQL path in the API request so local development needs one process. Set it false when the worker process is deployed.
