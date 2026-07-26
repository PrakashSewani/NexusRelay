#!/bin/sh
set -eu

output=${1-}
[ -n "$output" ] || { printf '%s\n' 'usage: generate-corefile.sh OUTPUT' >&2; exit 2; }

validate_host() {
  case "$1" in
    ''|*[!A-Za-z0-9.-]*|.*|*..*|*.) return 1 ;;
  esac
}

validate_ipv4() {
  old_ifs=$IFS
  IFS=.
  set -- $1
  IFS=$old_ifs
  [ "$#" -eq 4 ] || return 1
  for octet in "$@"; do
    case "$octet" in ''|*[!0-9]*) return 1 ;; esac
    [ "$octet" -le 255 ] || return 1
  done
}

admin_host=${ADMIN_HOST-}
private_traefik_ip=${PRIVATE_TRAEFIK_IP-}
private_dns_ip=${PRIVATE_DNS_IP-}

validate_host "$admin_host" || { printf '%s\n' 'ADMIN_HOST has invalid hostname syntax' >&2; exit 1; }
validate_ipv4 "$private_traefik_ip" || { printf '%s\n' 'PRIVATE_TRAEFIK_IP must be an IPv4 address' >&2; exit 1; }
validate_ipv4 "$private_dns_ip" || { printf '%s\n' 'PRIVATE_DNS_IP must be an IPv4 address' >&2; exit 1; }
admin_host_pattern=$(printf '%s' "$admin_host" | sed 's/\./[.]/g')

umask 077
cat >"$output" <<EOF
.:53 {
  bind $private_dns_ip
  errors
  health $private_dns_ip:8080
  ready $private_dns_ip:8181
  template IN A {
    match ^${admin_host_pattern}[.]$
    answer "{{ .Name }} 60 IN A $private_traefik_ip"
    fallthrough
  }
  template ANY ANY {
    rcode NXDOMAIN
  }
}
EOF
