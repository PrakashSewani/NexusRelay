#!/bin/sh
set -eu
set +x

IMAGE=${NEXUSRELAY_MIGRATE_IMAGE:-nexusrelay-migrate:local}
REDACTION_PROBE='NexusRelayValidationProbe-42'
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/nexusrelay-migrate-validation.XXXXXX")
chmod 0755 "$TMP_DIR"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT HUP INT TERM

cat > "$TMP_DIR/atlas" <<'EOF'
#!/bin/busybox sh
exit 0
EOF
chmod 0555 "$TMP_DIR/atlas"

printf '%s\n' "$REDACTION_PROBE" > "$TMP_DIR/valid-password"
printf '%s\r\n' "$REDACTION_PROBE" > "$TMP_DIR/valid-password-crlf"
: > "$TMP_DIR/empty-password"
printf ' %s\n' "$REDACTION_PROBE" > "$TMP_DIR/leading-space-password"
printf '%s \n' "$REDACTION_PROBE" > "$TMP_DIR/trailing-space-password"
printf '%s\n\n' "$REDACTION_PROBE" > "$TMP_DIR/double-newline-password"
ln -s valid-password "$TMP_DIR/symlink-password"
dd if=/dev/zero bs=4097 count=1 2>/dev/null | tr '\0' x > "$TMP_DIR/oversize-password"
printf '\r\n' >> "$TMP_DIR/oversize-password"
chmod 0444 "$TMP_DIR"/*-password

pass_count=0

run_case() {
  name=$1
  expected=$2
  host=$3
  port=$4
  database=$5
  user=$6
  sslmode=$7
  password_path=$8

  set +e
  output=$(docker run --rm \
    --network none \
    --read-only \
    --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
    --mount "type=bind,src=$TMP_DIR/atlas,dst=/atlas,readonly" \
    --mount "type=bind,src=$TMP_DIR,dst=/run/validation,readonly" \
    --env "DATABASE_HOST=$host" \
    --env "DATABASE_PORT=$port" \
    --env "DATABASE_NAME=$database" \
    --env "DATABASE_MIGRATION_USER=$user" \
    --env "DATABASE_MIGRATION_PASSWORD_FILE=$password_path" \
    --env "DATABASE_SSLMODE=$sslmode" \
    "$IMAGE" 2>&1)
  code=$?
  set -e

  case "$output" in
    *"$REDACTION_PROBE"*)
      printf 'FAIL %-36s secret appeared in output\n' "$name" >&2
      exit 1
      ;;
  esac

  if [ "$expected" = accept ]; then
    if [ "$code" -ne 0 ]; then
      printf 'FAIL %-36s expected acceptance, exit=%s output=%s\n' "$name" "$code" "$output" >&2
      exit 1
    fi
  elif [ "$code" -ne 2 ]; then
    printf 'FAIL %-36s expected validation exit 2, exit=%s output=%s\n' "$name" "$code" "$output" >&2
    exit 1
  fi

  pass_count=$((pass_count + 1))
  printf 'PASS %s\n' "$name"
}

valid_password=/run/validation/valid-password

# Host validation, including both forms of an IPv4-embedded IPv6 literal.
run_case host_ipv4 accept 127.0.0.1 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_dns accept postgres.internal 5432 nexusrelay nexusrelay_migration require "$valid_password"
run_case host_ipv6_compressed_embedded accept ::ffff:192.0.2.128 5432 nexusrelay nexusrelay_migration verify-full "$valid_password"
run_case host_ipv6_uncompressed_embedded accept 1:2:3:4:5:6:192.0.2.1 5432 nexusrelay nexusrelay_migration verify-ca "$valid_password"
run_case host_ipv6_too_many_groups reject 1:2:3:4:5:6:7:192.0.2.1 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_ipv6_too_few_groups reject 1:2:3:4:5:192.0.2.1 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_ipv6_bad_ipv4 reject 1:2:3:4:5:6:256.0.2.1 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_ipv6_bad_ipv4_zero reject ::ffff:192.00.2.1 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_ipv6_multiple_compression reject 1::2::192.0.2.1 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_ipv6_bracketed reject '[::ffff:192.0.2.128]' 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_ipv6_zone reject 'fe80::1%eth0' 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_dns_empty_label reject bad..host 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_url_delimiter reject postgres/path 5432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case host_ipv4_out_of_range reject 256.0.0.1 5432 nexusrelay nexusrelay_migration disable "$valid_password"

# Port, TLS mode, fixed identity, and fixed database validation.
run_case port_min accept postgres 1 nexusrelay nexusrelay_migration disable "$valid_password"
run_case port_max accept postgres 65535 nexusrelay nexusrelay_migration disable "$valid_password"
run_case port_zero reject postgres 0 nexusrelay nexusrelay_migration disable "$valid_password"
run_case port_overflow reject postgres 65536 nexusrelay nexusrelay_migration disable "$valid_password"
run_case port_leading_zero reject postgres 05432 nexusrelay nexusrelay_migration disable "$valid_password"
run_case ssl_allow_rejected reject postgres 5432 nexusrelay nexusrelay_migration allow "$valid_password"
run_case ssl_prefer_rejected reject postgres 5432 nexusrelay nexusrelay_migration prefer "$valid_password"
run_case user_wrong reject postgres 5432 nexusrelay postgres disable "$valid_password"
run_case database_wrong reject postgres 5432 other nexusrelay_migration disable "$valid_password"

# Password-file validation.
run_case password_lf accept postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/valid-password
run_case password_crlf accept postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/valid-password-crlf
run_case password_empty reject postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/empty-password
run_case password_leading_space reject postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/leading-space-password
run_case password_trailing_space reject postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/trailing-space-password
run_case password_double_newline reject postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/double-newline-password
run_case password_symlink reject postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/symlink-password
run_case password_oversize reject postgres 5432 nexusrelay nexusrelay_migration disable /run/validation/oversize-password
run_case password_relative_path reject postgres 5432 nexusrelay nexusrelay_migration disable run/validation/valid-password

printf 'Validation harness passed %s cases.\n' "$pass_count"
