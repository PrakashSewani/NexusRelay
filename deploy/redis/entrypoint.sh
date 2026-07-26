#!/bin/sh
set -eu
set +x

password_file=${REDIS_PASSWORD_FILE:-/run/secrets/redis_password}
case "$password_file" in
  /*) ;;
  *) printf 'Redis startup error: REDIS_PASSWORD_FILE must be absolute\n' >&2; exit 1 ;;
esac
[ -f "$password_file" ] && [ ! -L "$password_file" ] || {
  printf 'Redis startup error: REDIS_PASSWORD_FILE must name a regular non-symlink file\n' >&2
  exit 1
}
mode=$(stat -c %a "$password_file")
case "$mode" in
  400|600) ;;
  *) printf 'Redis startup error: REDIS_PASSWORD_FILE must not be accessible by group or other users\n' >&2; exit 1 ;;
esac
password=$(od -An -v -tu1 "$password_file" | awk '
  {
    for (i = 1; i <= NF; i++) bytes[++n] = $i
  }
  END {
    if (n < 1 || n > 4098) exit 1
    if (bytes[n] == 10) {
      n--
      if (n > 0 && bytes[n] == 13) n--
    }
    if (n < 1 || n > 4096) exit 1
    if (bytes[n] == 10 || bytes[n] == 13) exit 1
    for (i = 1; i <= n; i++) printf "%c", bytes[i]
  }
') || {
  printf 'Redis startup error: REDIS_PASSWORD_FILE has invalid size or newline format\n' >&2
  exit 1
}
case "$password" in
  ''|*[!A-Za-z0-9_-]*)
    printf 'Redis startup error: REDIS_PASSWORD_FILE must contain only base64url characters\n' >&2
    exit 1
    ;;
esac
[ "${#password}" -ge 32 ] || {
  printf 'Redis startup error: REDIS_PASSWORD_FILE must contain at least 32 characters\n' >&2
  exit 1
}

umask 077
cat > /tmp/redis.conf <<EOF
bind 0.0.0.0
protected-mode yes
port 6379
save ""
appendonly no
maxmemory-policy noeviction
aclfile /tmp/users.acl
EOF
# Phase 2 services only prove authenticated reachability. Later Redis Function
# work replaces this shared ping-only user with process-specific ACL identities.
printf 'user default on >%s resetkeys resetchannels -@all +ping\n' "$password" > /tmp/users.acl
unset password

exec redis-server /tmp/redis.conf
