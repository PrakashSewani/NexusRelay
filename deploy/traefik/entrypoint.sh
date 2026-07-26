#!/bin/sh
set -eu

validate_host() {
  setting=$1
  eval "host=\${$setting-}"
  case "$host" in
    ''|*[!A-Za-z0-9.-]*|.*|*..*|*.)
      printf 'Traefik startup error: %s has invalid hostname syntax\n' "$setting" >&2
      exit 1
      ;;
  esac
}

validate_host PUBLIC_API_HOST
validate_host ADMIN_HOST

tls_mode=${TLS_MODE-disabled}
case "$tls_mode" in
  disabled|acme) ;;
  *)
    printf 'Traefik startup error: TLS_MODE=%s is unsupported by this Compose profile\n' "$tls_mode" >&2
    exit 1
    ;;
esac

public_entrypoint=web
public_tls=''
if [ "$tls_mode" = acme ]; then
  [ "${ACME_DNS_PROVIDER-}" = cloudflare ] || {
    printf 'Traefik startup error: ACME_DNS_PROVIDER must be cloudflare in the Cloudflare profile\n' >&2
    exit 1
  }
  case "${ACME_EMAIL-}" in
    ?*@?*) ;;
    *) printf 'Traefik startup error: ACME_EMAIL is required in acme mode\n' >&2; exit 1 ;;
  esac
  public_entrypoint=websecure
  public_tls='      tls:
        certResolver: cloudflare'
fi

cat > /tmp/dynamic.yaml <<EOF
http:
  routers:
    gateway:
      entryPoints: [$public_entrypoint]
      rule: "Host(\`${PUBLIC_API_HOST}\`) && (Path(\`/v1\`) || PathPrefix(\`/v1/\`))"
      priority: 30
      service: gateway
$public_tls
    control-plane:
      entryPoints: [web]
      rule: "Host(\`${ADMIN_HOST}\`) && (Path(\`/api/control/v1\`) || PathPrefix(\`/api/control/v1/\`))"
      priority: 30
      service: control-plane
    web:
      entryPoints: [web]
      rule: "Host(\`${ADMIN_HOST}\`)"
      priority: 10
      service: web
EOF

if [ "$PUBLIC_API_HOST" != "$ADMIN_HOST" ]; then
  cat >> /tmp/dynamic.yaml <<EOF
    public-catchall:
      entryPoints: [$public_entrypoint]
      rule: "Host(\`${PUBLIC_API_HOST}\`)"
      priority: 1
      service: public-not-found
      middlewares: [public-not-found]
$public_tls
EOF
fi

cat >> /tmp/dynamic.yaml <<'EOF'
  middlewares:
    public-not-found:
      replacePath:
        path: "/__nexusrelay_not_found__"
  services:
    public-not-found:
      loadBalancer:
        servers:
          - url: "http://web:3000"
    gateway:
      loadBalancer:
        healthCheck:
          path: "/health/ready"
          interval: "5s"
          timeout: "3s"
        passHostHeader: true
        servers:
          - url: "http://gateway:8080"
    control-plane:
      loadBalancer:
        healthCheck:
          path: "/health/ready"
          interval: "5s"
          timeout: "3s"
        passHostHeader: true
        servers:
          - url: "http://control-plane:8080"
    web:
      loadBalancer:
        healthCheck:
          path: "/health/ready"
          interval: "5s"
          timeout: "3s"
        passHostHeader: true
        servers:
          - url: "http://web:3000"
EOF

if [ "$tls_mode" = acme ]; then
  cat >> /tmp/dynamic.yaml <<'EOF'
tls:
  options:
    default:
      minVersion: VersionTLS12
      sniStrict: true
EOF
fi

set -- traefik --configFile=/etc/traefik/traefik.yaml
if [ "$tls_mode" = acme ]; then
  set -- "$@" \
    --certificatesresolvers.cloudflare.acme.email="$ACME_EMAIL" \
    --certificatesresolvers.cloudflare.acme.storage=/var/lib/traefik/acme.json \
    --certificatesresolvers.cloudflare.acme.dnschallenge=true \
    --certificatesresolvers.cloudflare.acme.dnschallenge.provider=cloudflare
fi
exec "$@"
