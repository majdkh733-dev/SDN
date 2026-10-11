# ADR 004 — Secret references and a narrow adapter boundary

Device credentials are not a column of secret material. `CreateConnector` rejects a posted secret and stores `vault://{organization}/{connector}/management`. The in-process manager only formats that reference. A deployment can replace it with a Vault client without changing the HTTP contract.

The adapter boundary is `internal/adapter` plus `connectors/ovn`. The simulator is the verified implementation. OVN is the one real southbound, limited to logical-switch listing through `ovn-nbctl`, and it is not lab-verified here. Additional vendors stay in the integration catalog with status `not_implemented` so the console does not imply support.
