#!/bin/sh
# Reports the OVN/OVS adapter status for this machine.
# Exit 0 in every case. The status line is the result.
set -eu
if ! command -v ovn-nbctl >/dev/null 2>&1; then
  echo "status=implemented_unverified detail=ovn-nbctl is not installed"
  exit 0
fi
if ! ovn-nbctl --version >/dev/null 2>&1; then
  echo "status=implemented_unverified detail=ovn-nbctl --version failed"
  exit 0
fi
if [ -n "${OVN_NB_DB:-}" ]; then
  if ovn-nbctl --db="$OVN_NB_DB" ls-list >/dev/null 2>&1; then
    echo "status=lab_verified detail=ovn-nbctl ls-list succeeded"
    exit 0
  fi
  echo "status=implemented_unverified detail=ovn-nbctl could not list logical switches at OVN_NB_DB"
  exit 0
fi
if ovn-nbctl ls-list >/dev/null 2>&1; then
  echo "status=lab_verified detail=ovn-nbctl ls-list succeeded against the default northbound database"
  exit 0
fi
echo "status=implemented_unverified detail=ovn-nbctl is installed and no northbound database answered"
