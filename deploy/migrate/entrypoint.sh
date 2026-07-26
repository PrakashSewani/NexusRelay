#!/bin/busybox sh
set -eu
set +x

fail() {
  printf 'migration configuration error: %s\n' "$1" >&2
  exit 2
}

required() {
  setting=$1
  eval "value=\${$setting-}"
  [ -n "$value" ] || fail "$setting is required"
}

validate_host() {
  host=$1
  host_bytes=$(printf %s "$host" | /bin/busybox wc -c)
  [ "$host_bytes" -gt 0 ] || fail 'DATABASE_HOST is required'
  printf %s "$host" | /bin/busybox od -An -v -tu1 | /bin/busybox awk '
    {
      for (i = 1; i <= NF; i++) if ($i < 33 || $i > 126) exit 1
    }
  ' || fail 'DATABASE_HOST contains whitespace, control, or non-ASCII characters'

  case "$host" in
    *:*)
      [ "$host_bytes" -le 45 ] || fail 'DATABASE_HOST is not a valid IPv6 literal'
      printf %s "$host" | /bin/busybox awk '
        function ipv4(s, a, n, i) {
          n = split(s, a, ".")
          if (n != 4) return 0
          for (i = 1; i <= 4; i++) {
            if (a[i] !~ /^[0-9]+$/ || (length(a[i]) > 1 && substr(a[i], 1, 1) == "0") || a[i] + 0 > 255) return 0
          }
          return 1
        }
        function groups(s, a, n, i) {
          if (s == "") return 0
          n = split(s, a, ":")
          for (i = 1; i <= n; i++) if (a[i] !~ /^[0-9A-Fa-f]{1,4}$/) return -1
          return n
        }
        {
          s = $0
          if (s !~ /^[0-9A-Fa-f:.]+$/) exit 1
          if (s ~ /\./) {
            embedded_ipv4 = 1
            p = 0
            for (i = 1; i <= length(s); i++) if (substr(s, i, 1) == ":") p = i
            if (p == 0 || !ipv4(substr(s, p + 1))) exit 1
            s = substr(s, 1, p) "0:0"
          }
          p = index(s, "::")
          if (p > 0) {
            if (index(substr(s, p + 2), "::") > 0) exit 1
            left = groups(substr(s, 1, p - 1))
            right = groups(substr(s, p + 2))
            if (left < 0 || right < 0 || left + right >= 8) exit 1
          } else if (groups(s) != 8) exit 1
        }
      ' || fail 'DATABASE_HOST is not a valid IPv6 literal'
      DATABASE_HOST_URL="[$host]"
      ;;
    *[!0-9.]*|'')
      [ "$host_bytes" -le 253 ] || fail 'DATABASE_HOST exceeds 253 bytes'
      printf %s "$host" | /bin/busybox awk -F. '
        NF < 1 { exit 1 }
        {
          for (i = 1; i <= NF; i++) {
            if (length($i) < 1 || length($i) > 63 || $i !~ /^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$/) exit 1
          }
        }
      ' || fail 'DATABASE_HOST is not a valid DNS hostname'
      DATABASE_HOST_URL=$host
      ;;
    *)
      [ "$host_bytes" -le 15 ] || fail 'DATABASE_HOST is not a valid IPv4 literal'
      printf %s "$host" | /bin/busybox awk -F. '
        NF != 4 { exit 1 }
        {
          for (i = 1; i <= 4; i++) {
            if ($i !~ /^[0-9]+$/ || (length($i) > 1 && substr($i, 1, 1) == "0") || $i + 0 > 255) exit 1
          }
        }
      ' || fail 'DATABASE_HOST is not a valid IPv4 literal'
      DATABASE_HOST_URL=$host
      ;;
  esac
  export DATABASE_HOST_URL
}

validate_password_file() {
  password_file=$1
  case "$password_file" in
    /*) ;;
    *) fail 'DATABASE_MIGRATION_PASSWORD_FILE must be an absolute path' ;;
  esac
  [ -f "$password_file" ] || fail 'DATABASE_MIGRATION_PASSWORD_FILE must name a regular file'
  [ ! -L "$password_file" ] || fail 'DATABASE_MIGRATION_PASSWORD_FILE must not be a symbolic link'
  /bin/busybox od -An -v -tu1 "$password_file" | /bin/busybox awk '
    {
      for (i = 1; i <= NF; i++) bytes[++n] = $i
    }
    END {
      if (n < 1 || n > 4098) exit 1
      content = n
      if (bytes[content] == 10) {
        content--
        if (content > 0 && bytes[content] == 13) content--
      }
      if (content < 1 || content > 4096) exit 1
      if (bytes[content] == 10 || bytes[content] == 13) exit 1
      if (bytes[1] == 9 || bytes[1] == 10 || bytes[1] == 11 || bytes[1] == 12 || bytes[1] == 13 || bytes[1] == 32) exit 1
      if (bytes[content] == 9 || bytes[content] == 10 || bytes[content] == 11 || bytes[content] == 12 || bytes[content] == 13 || bytes[content] == 32) exit 1
    }
  ' || fail 'DATABASE_MIGRATION_PASSWORD_FILE has an invalid size, newline, or surrounding-whitespace format'
}

required DATABASE_HOST
required DATABASE_PORT
required DATABASE_NAME
required DATABASE_MIGRATION_USER
required DATABASE_MIGRATION_PASSWORD_FILE
required DATABASE_SSLMODE

[ "$DATABASE_MIGRATION_USER" = nexusrelay_migration ] || fail 'DATABASE_MIGRATION_USER must be nexusrelay_migration'
[ "$DATABASE_NAME" = nexusrelay ] || fail 'DATABASE_NAME must be nexusrelay'

case "$DATABASE_PORT" in
  *[!0-9]*|'') fail 'DATABASE_PORT must be an integer from 1 through 65535' ;;
esac
[ "$DATABASE_PORT" -ge 1 ] 2>/dev/null && [ "$DATABASE_PORT" -le 65535 ] 2>/dev/null || fail 'DATABASE_PORT must be an integer from 1 through 65535'
[ "$DATABASE_PORT" = "$((DATABASE_PORT + 0))" ] || fail 'DATABASE_PORT must use canonical decimal notation'

case "$DATABASE_SSLMODE" in
  disable|require|verify-ca|verify-full) ;;
  *) fail 'DATABASE_SSLMODE must be disable, require, verify-ca, or verify-full' ;;
esac

validate_host "$DATABASE_HOST"
validate_password_file "$DATABASE_MIGRATION_PASSWORD_FILE"

/atlas migrate validate --dir file://migrations
exec /atlas migrate apply \
  --config file://atlas.hcl \
  --env nexusrelay \
  --tx-mode file \
  --exec-order linear \
  --lock-timeout 60s
