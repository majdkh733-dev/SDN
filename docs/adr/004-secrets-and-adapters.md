# ADR 004 — Secret references and a narrow adapter boundary

Device credentials are not a column of secret material. `CreateConnector` rejects a posted secret and stores `vault://{organization}/{connector}/management`.

`internal/secrets` has three backends. `memory` is the development default. `file` writes mode `0600` files whose names are a hash of the reference. `vault` is an HTTP client for KV v2: it posts and reads `{VAULT_ADDR}/v1/{VAULT_MOUNT}/data/{org}/{connector}/management` with the value under `value`. `TestVaultKV` uses an `httptest` server. No Vault process is configured in this environment. `TestProductionRequiresVaultAndRejectsDevShortcuts` checks that production startup refuses `memory` and `file`.

The adapter boundary is `internal/adapter` plus `connectors/ovn`. The simulator is the verified implementation. OVN is the one real southbound, limited to logical-switch listing through `ovn-nbctl`. `TestLabProbe` and `scripts/ovn-lab-check.sh` both reported `implemented_unverified` because `ovn-nbctl` is not installed. Additional vendors stay in the integration catalog with status `not_implemented` so the console does not imply support.
