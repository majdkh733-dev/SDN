# ADR 002 — PostgreSQL fencing, no etcd

etcd would be justified by a need for a watchable coordination log that several controllers must observe with the same key history. This release has one write authority per execution domain and already uses PostgreSQL as the system of record.

`authority_epochs` stores one epoch per organization and execution domain. A worker updates the epoch while it holds the domain. The console reports etcd as `not_deployed`.

This does not fence a device session that was opened before the epoch moved. The OVN adapter, when a lab exists, still has to use the controller's own locking. That limitation is shown on the Controllers page.
