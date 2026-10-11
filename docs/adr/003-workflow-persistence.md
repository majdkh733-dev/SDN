# ADR 003 — Durable workflows in PostgreSQL

Temporal is the intended durable workflow engine when the deployment needs its replay and visibility tooling. It is not imported in this repository, and Compose does not start it. Claiming that Temporal executes these changes would be false.

What is implemented:

- `workflow_runs` and `execution_steps` persist each attempt.
- Steps are unique per change and operation key, so a retry continues instead of repeating a completed step.
- `RunChange` takes a PostgreSQL advisory lock on the change id.
- An uncertain device result stops the pass. The next run inspects `simulator_acls` before inserting again.
- Cancellation is a flag the runner observes between devices.
- Failure, partial apply, and rollback failure are stored on the change and returned to the console.

`INLINE_WORKFLOWS=true` runs that path in the API request so local development needs one process. Set it false when the worker process is deployed.
