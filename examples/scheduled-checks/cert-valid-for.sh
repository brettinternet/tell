#!/bin/sh
# Usage: cert-valid-for.sh DAYS HOST[:PORT]   exit 0 if HOST's TLS certificate outlives DAYS.
set -u
case $2 in *:*) target=$2 ;; *) target=$2:443 ;; esac
openssl s_client -connect "$target" -servername "${2%%:*}" </dev/null 2>/dev/null |
    openssl x509 -noout -checkend $(($1 * 86400)) >/dev/null
