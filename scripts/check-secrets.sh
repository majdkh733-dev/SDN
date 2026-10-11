#!/bin/sh
set -eu
# Fails when committed source contains key material or cloud secret prefixes.
if git grep -n -I -E 'BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY|AKIA[0-9A-Z]{16}|sk_live_[0-9A-Za-z]' -- . ':(exclude)apps/web/node_modules' ':(exclude)scripts/check-secrets.sh'; then
  echo "refusing: possible secret material in the tree" >&2
  exit 1
fi
echo "secret scan found no key material"
