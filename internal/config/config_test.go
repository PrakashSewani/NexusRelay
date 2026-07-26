package config

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

const sentinel = "do-not-disclose-this-secret"

var reservedTailscaleSettings = []string{
	"TAILSCALE_AUTH_KEY_FILE",
	"TAILSCALE_STATE_DIR",
	"TAILSCALE_HOSTNAME",
	"PRIVATE_INGRESS_SUBNET",
	"PRIVATE_TRAEFIK_IP",
	"PRIVATE_DNS_IP",
	"TAILSCALE_ADVERTISE_ROUTES",
}

func TestEnvInventoryIsExactAndClassified(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), ".env.example")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	keys := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, _, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid inventory line %q", line)
		}
		if keys[name] {
			t.Fatalf("duplicate inventory key %s", name)
		}
		keys[name] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 113 {
		t.Fatalf("inventory count = %d, want 113", len(keys))
	}
	if len(inventory) != len(keys) {
		t.Fatalf("typed inventory count = %d, .env count = %d", len(inventory), len(keys))
	}
	for name := range keys {
		class, ok := inventory[name]
		if !ok {
			t.Errorf("%s is not typed", name)
			continue
		}
		if len(class.Consumers) == 0 && !slices.Contains(reservedTailscaleSettings, name) {
			t.Errorf("%s has no consumer classification", name)
		}
		if strings.HasSuffix(name, "_FILE") && !class.Secret && name != "TLS_CERT_FILE" {
			t.Errorf("%s is not classified secret", name)
		}
	}
	for name := range inventory {
		if !keys[name] {
			t.Errorf("typed setting %s is absent from .env.example", name)
		}
	}
}

func TestEnvironmentIsExactEnum(t *testing.T) {
	for _, environment := range []string{"development", "test"} {
		values := fixture(t)
		values["NEXUSRELAY_ENV"] = environment
		if _, err := ParseGateway(values); err != nil {
			t.Errorf("%s rejected: %v", environment, err)
		}
	}
	for _, environment := range []string{"prod", "Production", "staging", "unknown", ""} {
		values := fixture(t)
		values["NEXUSRELAY_ENV"] = environment
		if _, err := ParseGateway(values); err == nil {
			t.Errorf("%q accepted", environment)
		}
	}
}

func TestInternalHTTPBindDefault(t *testing.T) {
	settings, err := ParseGateway(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if settings.HTTP.Address != "0.0.0.0:8080" {
		t.Fatalf("HTTP address = %q", settings.HTTP.Address)
	}
}

func TestProcessConfigurationsAndCredentialSeparation(t *testing.T) {
	values := fixture(t)
	gateway, err := ParseGateway(values)
	if err != nil {
		t.Fatal(err)
	}
	if gateway.Database.User != gatewayUser || gateway.Redis.MaxConnections != 50 || gateway.Policy.UpstreamTotalTimeout != 10*time.Minute {
		t.Fatalf("unexpected gateway config: %+v", gateway)
	}
	control, err := ParseControlPlane(values)
	if err != nil {
		t.Fatal(err)
	}
	if control.Database.User != controlPlaneUser || control.Sessions.CSRFKeys.ActiveKeyID() != "csrf-1" {
		t.Fatalf("unexpected control-plane config: %+v", control)
	}
	worker, err := ParseWorker(values)
	if err != nil {
		t.Fatal(err)
	}
	if worker.Database.User != workerUser || worker.Policy.Concurrency != 4 {
		t.Fatalf("unexpected worker config: %+v", worker)
	}

	broken := clone(values)
	broken["DATABASE_CONTROL_PLANE_PASSWORD_FILE"] = filepath.Join(t.TempDir(), "missing")
	if _, err := ParseGateway(broken); err != nil {
		t.Fatalf("gateway read another process credential: %v", err)
	}
	broken = clone(values)
	broken["API_KEY_PEPPER_RING_FILE"] = filepath.Join(t.TempDir(), "missing")
	if _, err := ParseWorker(broken); err != nil {
		t.Fatalf("worker read API-key pepper ring: %v", err)
	}
	broken = clone(values)
	broken["SESSION_SECRET_FILE"] = filepath.Join(t.TempDir(), "missing")
	if _, err := ParseGateway(broken); err != nil {
		t.Fatalf("gateway read browser session secret: %v", err)
	}
}

func TestRuntimeDatabaseParsersIgnoreOtherPrincipalsAndPasswords(t *testing.T) {
	tests := []struct {
		name             string
		process          Process
		parse            func(map[string]string) error
		selectedUser     string
		selectedPassword string
		foreignUsers     []string
		foreignPasswords []string
	}{
		{name: "gateway", process: ProcessGateway, parse: func(v map[string]string) error { _, err := ParseGateway(v); return err }, selectedUser: "DATABASE_GATEWAY_USER", selectedPassword: "DATABASE_GATEWAY_PASSWORD_FILE", foreignUsers: []string{"POSTGRES_USER", "DATABASE_MIGRATION_USER", "DATABASE_CONTROL_PLANE_USER", "DATABASE_WORKER_USER"}, foreignPasswords: []string{"POSTGRES_PASSWORD_FILE", "DATABASE_MIGRATION_PASSWORD_FILE", "DATABASE_CONTROL_PLANE_PASSWORD_FILE", "DATABASE_WORKER_PASSWORD_FILE"}},
		{name: "control plane", process: ProcessControlPlane, parse: func(v map[string]string) error { _, err := ParseControlPlane(v); return err }, selectedUser: "DATABASE_CONTROL_PLANE_USER", selectedPassword: "DATABASE_CONTROL_PLANE_PASSWORD_FILE", foreignUsers: []string{"POSTGRES_USER", "DATABASE_MIGRATION_USER", "DATABASE_GATEWAY_USER", "DATABASE_WORKER_USER"}, foreignPasswords: []string{"POSTGRES_PASSWORD_FILE", "DATABASE_MIGRATION_PASSWORD_FILE", "DATABASE_GATEWAY_PASSWORD_FILE", "DATABASE_WORKER_PASSWORD_FILE"}},
		{name: "worker", process: ProcessWorker, parse: func(v map[string]string) error { _, err := ParseWorker(v); return err }, selectedUser: "DATABASE_WORKER_USER", selectedPassword: "DATABASE_WORKER_PASSWORD_FILE", foreignUsers: []string{"POSTGRES_USER", "DATABASE_MIGRATION_USER", "DATABASE_GATEWAY_USER", "DATABASE_CONTROL_PLANE_USER"}, foreignPasswords: []string{"POSTGRES_PASSWORD_FILE", "DATABASE_MIGRATION_PASSWORD_FILE", "DATABASE_GATEWAY_PASSWORD_FILE", "DATABASE_CONTROL_PLANE_PASSWORD_FILE"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			full := fixture(t)
			full[test.selectedUser] = map[string]string{
				"DATABASE_GATEWAY_USER":       gatewayUser,
				"DATABASE_CONTROL_PLANE_USER": controlPlaneUser,
				"DATABASE_WORKER_USER":        workerUser,
			}[test.selectedUser]
			values := ValuesForProcess(full, test.process)
			for _, name := range append(append([]string{}, test.foreignUsers...), test.foreignPasswords...) {
				if _, ok := values[name]; ok {
					t.Fatalf("process view contains unrelated credential %s", name)
				}
			}
			if err := test.parse(values); err != nil {
				t.Fatalf("process-filtered view rejected: %v", err)
			}

			values = clone(full)
			for _, name := range test.foreignUsers {
				values[name] = "invalid-foreign-user"
			}
			for _, name := range test.foreignPasswords {
				values[name] = filepath.Join(t.TempDir(), "missing")
			}
			if err := test.parse(values); err != nil {
				t.Fatalf("foreign credentials parsed: %v", err)
			}

			values = fixture(t)
			values[test.selectedUser] = "invalid-selected-user"
			if err := test.parse(values); err == nil || !strings.Contains(err.Error(), test.selectedUser) {
				t.Fatalf("selected user error = %v", err)
			}
			values = fixture(t)
			values[test.selectedPassword] = filepath.Join(t.TempDir(), "missing")
			if err := test.parse(values); err == nil || !strings.Contains(err.Error(), test.selectedPassword) {
				t.Fatalf("selected password error = %v", err)
			}
		})
	}
}

func TestLongRunningServicesRejectBootstrapSettings(t *testing.T) {
	for _, parse := range []func(map[string]string) error{
		func(v map[string]string) error { _, err := ParseGateway(v); return err },
		func(v map[string]string) error { _, err := ParseControlPlane(v); return err },
		func(v map[string]string) error { _, err := ParseWorker(v); return err },
	} {
		for _, name := range []string{"BOOTSTRAP_OWNER_EMAIL", "BOOTSTRAP_OWNER_NAME", "BOOTSTRAP_ORGANIZATION_NAME", "BOOTSTRAP_ORGANIZATION_SLUG", "BOOTSTRAP_PASSWORD_FILE"} {
			values := fixture(t)
			values[name] = ""
			if err := parse(values); err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("%s error = %v", name, err)
			}
		}
	}
}

func TestDeploymentValidationIgnoresCommandOnlyBootstrapSettings(t *testing.T) {
	values := fixture(t)
	values["BOOTSTRAP_OWNER_EMAIL"] = ""
	values["BOOTSTRAP_PASSWORD_FILE"] = filepath.Join(t.TempDir(), "must-not-read")
	if _, err := ParseDeployment(values); err != nil {
		t.Fatalf("deployment validator consumed command-only bootstrap settings: %v", err)
	}
}

func TestBootstrapUsesOnlyControlPlaneCredentialAndPasswordPolicy(t *testing.T) {
	values := fixture(t)
	values["BOOTSTRAP_OWNER_EMAIL"] = "owner@example.com"
	values["BOOTSTRAP_OWNER_NAME"] = "Owner"
	values["BOOTSTRAP_ORGANIZATION_NAME"] = "Example"
	values["BOOTSTRAP_ORGANIZATION_SLUG"] = "example"
	values["BOOTSTRAP_PASSWORD_FILE"] = writeSecret(t, "bootstrap", "correct horse battery staple", 0o600)
	values["SESSION_SECRET_FILE"] = filepath.Join(t.TempDir(), "must-not-read")
	values["CSRF_SECRET_RING_FILE"] = filepath.Join(t.TempDir(), "must-not-read")
	values["PUBLIC_API_BASE_URL"] = "not-a-url"
	values["TLS_MODE"] = "invalid"
	config, err := ParseBootstrap(values)
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.User != controlPlaneUser || config.Password.Reveal() != "correct horse battery staple" {
		t.Fatalf("unexpected bootstrap config: %+v", config)
	}
	values["DATABASE_GATEWAY_PASSWORD_FILE"] = filepath.Join(t.TempDir(), "must-not-read")
	if _, err := ParseBootstrap(values); err != nil {
		t.Fatalf("bootstrap read gateway credential: %v", err)
	}
}

func TestFixedDatabaseIdentityAndPoolValidation(t *testing.T) {
	for name, invalid := range map[string]string{
		"POSTGRES_DB": "other", "POSTGRES_USER": "postgres", "DATABASE_MIGRATION_USER": "migration", "DATABASE_GATEWAY_USER": "gateway", "DATABASE_CONTROL_PLANE_USER": "control", "DATABASE_WORKER_USER": "worker",
	} {
		values := fixture(t)
		values[name] = invalid
		if _, err := ParseDeployment(values); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s error = %v", name, err)
		}
	}
	values := fixture(t)
	values["DATABASE_MIN_CONNECTIONS_GATEWAY"] = "31"
	if _, err := ParseGateway(values); err == nil {
		t.Fatal("min > max accepted")
	}
	values = fixture(t)
	values["POSTGRES_PASSWORD_FILE"] = values["DATABASE_GATEWAY_PASSWORD_FILE"]
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("duplicate database password paths accepted")
	}
}

func TestURLsTLSOriginsAndTunnelValidation(t *testing.T) {
	for name, test := range map[string]struct {
		invalid string
		parse   func(map[string]string) error
	}{
		"PUBLIC_API_BASE_URL": {"https://api.example.com", func(v map[string]string) error { _, err := ParseGateway(v); return err }},
		"ADMIN_BASE_URL":      {"https://admin.example.com/path", func(v map[string]string) error { _, err := ParseControlPlane(v); return err }},
		"ADMIN_ORIGINS":       {"https://admin.example.com/path", func(v map[string]string) error { _, err := ParseControlPlane(v); return err }},
		"TRUSTED_PROXY_CIDRS": {"127.0.0.1", func(v map[string]string) error { _, err := ParseGateway(v); return err }},
		"PUBLIC_API_HOST":     {"wrong.example.com", func(v map[string]string) error { _, err := ParseDeployment(v); return err }},
	} {
		values := fixture(t)
		values[name] = test.invalid
		if err := test.parse(values); err == nil {
			t.Errorf("%s=%q accepted", name, test.invalid)
		}
	}
	values := fixture(t)
	values["NEXUSRELAY_ENV"] = "production"
	if _, err := ParseGateway(values); err == nil {
		t.Fatal("production HTTP URLs accepted")
	}
	values = productionFixture(t)
	values["ADMIN_ORIGINS"] = "https://admin.example.com,http://other.example.com"
	if _, err := ParseControlPlane(values); err == nil {
		t.Fatal("production HTTP admin origin accepted")
	}
	values = fixture(t)
	values["TLS_MODE"] = "files"
	values["TLS_CERT_FILE"] = filepath.Join(t.TempDir(), "missing")
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("missing TLS files accepted")
	}
	values = fixture(t)
	values["ENABLE_CLOUDFLARE_TUNNEL"] = "true"
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("HTTP Cloudflare URL accepted")
	}
	values = productionFixture(t)
	values["ENABLE_CLOUDFLARE_TUNNEL"] = "true"
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("Cloudflare tunnel without an HTTPS Traefik origin accepted")
	}
	values = fixture(t)
	values["PUBLIC_API_BASE_URL"] = "https://api.example.com/v1"
	values["PUBLIC_API_HOST"] = "api.example.com"
	values["ADMIN_BASE_URL"] = "https://admin.example.com"
	values["ADMIN_HOST"] = "admin.example.com"
	values["ADMIN_ORIGINS"] = "https://admin.example.com"
	values["ENABLE_CLOUDFLARE_TUNNEL"] = "true"
	values["TLS_MODE"] = "acme"
	values["ACME_EMAIL"] = "operator@example.com"
	values["ACME_DNS_PROVIDER"] = "cloudflare"
	values["ACME_DNS_API_TOKEN_FILE"] = writeSecret(t, "acme", "token", 0o600)
	values["CLOUDFLARE_TUNNEL_ID"] = "tunnel-1"
	values["CLOUDFLARE_TUNNEL_CREDENTIALS_FILE"] = writeSecret(t, "cloudflare", `{"AccountTag":"account","TunnelSecret":"secret","TunnelID":"other"}`, 0o600)
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("mismatched Cloudflare credentials accepted")
	}
	values = productionFixture(t)
	values["ENABLE_CLOUDFLARE_TUNNEL"] = "true"
	values["TLS_MODE"] = "acme"
	values["ACME_EMAIL"] = "operator@example.com"
	values["ACME_DNS_PROVIDER"] = "cloudflare"
	values["ACME_DNS_API_TOKEN_FILE"] = writeSecret(t, "acme", "token", 0o600)
	values["ADMIN_EXPOSURE_MODE"] = "private"
	values["ADMIN_BASE_URL"] = "https://api.example.com"
	values["ADMIN_HOST"] = "api.example.com"
	values["ADMIN_ORIGINS"] = "https://api.example.com"
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("private admin host published by Cloudflare")
	}
	values["ADMIN_EXPOSURE_MODE"] = "public"
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("shared public/admin host published by Cloudflare")
	}
	values = fixture(t)
	values["ENABLE_TAILSCALE_PRIVATE_ADMIN"] = "true"
	if _, err := ParseDeployment(values); err == nil || !strings.Contains(err.Error(), "ADR 0002") {
		t.Fatalf("Tailscale error = %v", err)
	}
}

func TestOTLPHTTPCollectorEndpointValidation(t *testing.T) {
	for _, endpoint := range []string{
		"grpc://collector:4317",
		"https://user:password@collector.example/v1/traces",
		"https://collector.example/v1/traces?token=secret",
		"https://collector.example/v1/traces#fragment",
	} {
		values := fixture(t)
		values["OTEL_ENABLED"] = "true"
		values["OTEL_EXPORTER_OTLP_ENDPOINT"] = endpoint
		if _, err := ParseGateway(values); err == nil {
			t.Errorf("OTLP endpoint %q accepted", endpoint)
		}
	}
	values := fixture(t)
	values["OTEL_ENABLED"] = "true"
	values["OTEL_EXPORTER_OTLP_ENDPOINT"] = "https://collector.example/v1/traces"
	settings, err := ParseGateway(values)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Observability.OTLPEndpoint.String() != values["OTEL_EXPORTER_OTLP_ENDPOINT"] {
		t.Fatalf("OTLP endpoint = %q", settings.Observability.OTLPEndpoint)
	}
	values = fixture(t)
	values["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://127.0.0.1:4318/v1/traces"
	settings, err = ParseGateway(values)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Observability.OTel {
		t.Fatal("collector endpoint enabled tracing implicitly")
	}
}

func TestTailscaleDisabledIgnoresConditionalValues(t *testing.T) {
	values := fixture(t)
	values["ENABLE_TAILSCALE_PRIVATE_ADMIN"] = "false"
	values["PRIVATE_INGRESS_SUBNET"] = "not-a-cidr"
	values["PRIVATE_TRAEFIK_IP"] = "not-an-ip"
	values["TAILSCALE_AUTH_KEY_FILE"] = filepath.Join(t.TempDir(), "missing")
	if _, err := ParseDeployment(values); err != nil {
		t.Fatalf("disabled Tailscale parsed conditional settings: %v", err)
	}
	values["ENABLE_TAILSCALE_PRIVATE_ADMIN"] = "true"
	if _, err := ParseDeployment(values); err == nil || !strings.Contains(err.Error(), "ADR 0002") {
		t.Fatalf("enabled Tailscale error = %v", err)
	}
}

func TestDatabaseSSLModePolicy(t *testing.T) {
	for _, mode := range []string{"allow", "prefer"} {
		values := fixture(t)
		values["DATABASE_SSLMODE"] = mode
		if _, err := ParseGateway(values); err == nil {
			t.Errorf("DATABASE_SSLMODE=%s accepted", mode)
		}
	}
	values := productionFixture(t)
	values["DATABASE_SSLMODE"] = "disable"
	if _, err := ParseGateway(values); err == nil {
		t.Fatal("production DATABASE_SSLMODE=disable accepted")
	}
	for _, mode := range []string{"require", "verify-ca", "verify-full"} {
		values := productionFixture(t)
		values["DATABASE_SSLMODE"] = mode
		if _, err := ParseGateway(values); err != nil {
			t.Errorf("production DATABASE_SSLMODE=%s rejected: %v", mode, err)
		}
	}
}

func TestTimeoutWorkerAndArgon2Boundaries(t *testing.T) {
	for name, invalid := range map[string]string{
		"UPSTREAM_TOTAL_TIMEOUT": "5s", "UPSTREAM_CONNECT_TIMEOUT": "11m", "MAX_UPSTREAM_ATTEMPTS": "0", "REQUEST_BODY_MAX_BYTES": "0",
	} {
		values := fixture(t)
		values[name] = invalid
		if _, err := ParseGateway(values); err == nil {
			t.Errorf("%s=%q accepted", name, invalid)
		}
	}
	for name, invalid := range map[string]string{
		"OUTBOX_RETRY_BASE_DELAY": "10m", "OUTBOX_RETRY_MAX_DELAY": "500ms", "HEALTH_FAST_WINDOW": "31m", "WORKER_CONCURRENCY": "0", "RETENTION_BATCH_SIZE": "0", "HEALTH_CONSECUTIVE_FAILURES": "0",
	} {
		values := fixture(t)
		values[name] = invalid
		if _, err := ParseWorker(values); err == nil {
			t.Errorf("%s=%q accepted", name, invalid)
		}
	}
	values := fixture(t)
	values["PASSWORD_ARGON2_MEMORY_KIB"] = "8"
	values["PASSWORD_ARGON2_PARALLELISM"] = "2"
	if _, err := ParseControlPlane(values); err == nil {
		t.Fatal("invalid Argon2 memory/parallelism accepted")
	}
	values = fixture(t)
	values["HEALTH_UNAVAILABLE_ENTRY_THRESHOLD_BPS"] = "9000"
	values["HEALTH_UNAVAILABLE_EXIT_THRESHOLD_BPS"] = "9000"
	if _, err := ParseWorker(values); err == nil {
		t.Fatal("health hysteresis without separation accepted")
	}
}

func TestStrictSecretFormatsAndRedaction(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	for name, value := range map[string]string{
		"duplicate top-level": `{"version":1,"version":1,"active_key_id":"k","keys":[{"key_id":"k","key":"` + key + `"}]}`,
		"duplicate nested":    `{"version":1,"active_key_id":"k","keys":[{"key_id":"k","key_id":"k2","key":"` + key + `"}]}`,
		"unknown":             `{"version":1,"active_key_id":"k","extra":true,"keys":[{"key_id":"k","key":"` + key + `"}]}`,
		"trailing":            ringJSON("k", key) + ` {}`,
		"unpadded":            ringJSON("k", strings.TrimRight(key, "=")),
		"wrong case":          `{"Version":1,"active_key_id":"k","keys":[{"key_id":"k","key":"` + key + `"}]}`,
		"mixed alias":         `{"version":1,"Version":1,"active_key_id":"k","keys":[{"key_id":"k","key":"` + key + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			for _, setting := range []string{"API_KEY_PEPPER_RING_FILE", "CSRF_SECRET_RING_FILE"} {
				path := writeSecret(t, "ring", value, 0o600)
				if _, err := ReadKeyRing(setting, path); err == nil {
					t.Fatalf("%s accepted invalid ring", setting)
				}
			}
		})
	}
	for _, value := range []string{
		`{"Version":1,"expected_epoch":1,"active_key_id":"k","keys":[{"key_id":"k","key":"` + key + `"}]}`,
		`{"version":1,"EXPECTED_EPOCH":1,"active_key_id":"k","keys":[{"key_id":"k","key":"` + key + `"}]}`,
		`{"version":1,"expected_epoch":1,"active_key_id":"k","keys":[{"Key_ID":"k","key":"` + key + `"}]}`,
	} {
		if _, err := ReadProviderKeyRing(writeSecret(t, "provider-case", value, 0o600)); err == nil {
			t.Errorf("provider ring accepted mixed-case fields: %s", value)
		}
	}
	path := writeSecret(t, "provider", `{"version":1,"expected_epoch":1,"active_key_id":"k","keys":[{"key_id":"k","key":"`+key+`"}]}`, 0o644)
	if _, err := ReadProviderKeyRing(path); err == nil {
		t.Fatal("group-readable provider ring accepted")
	}
	secret := Secret{value: sentinel}
	ring := KeyRing{activeKeyID: sentinel, keys: map[string][32]byte{sentinel: {}}}
	provider := ProviderKeyRing{KeyRing: ring, expectedEpoch: 1}
	composite := Gateway{Database: Database{Password: secret}, Redis: Redis{URL: secret}, ProviderKeys: provider, APIKeyPeppers: ring}
	for _, formatted := range []string{fmt.Sprint(secret), fmt.Sprintf("%#v", secret), fmt.Sprintf("%+v", ring), fmt.Sprintf("%+v", provider), fmt.Sprintf("%+v", composite)} {
		if strings.Contains(formatted, sentinel) {
			t.Fatalf("formatted value disclosed secret: %s", formatted)
		}
	}
}

func TestProtectedSecretFilePermissionsAndSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission semantics")
	}
	for _, mode := range []os.FileMode{0o640, 0o604, 0o666} {
		path := writeSecret(t, "permissive", "secret", mode)
		if _, err := ReadSimpleSecret("DATABASE_GATEWAY_PASSWORD_FILE", path, 4096); err == nil {
			t.Errorf("mode %o accepted", mode)
		}
	}
	directory := t.TempDir()
	target := writeAt(t, directory, "target", "secret", 0o600)
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSimpleSecret("REDIS_URL_FILE", link, 4096); err == nil {
		t.Fatal("secret symlink accepted")
	}
}

func TestTLSCertificateMayBePublicButPrivateKeyMustBeProtected(t *testing.T) {
	values := fixture(t)
	values["TLS_MODE"] = "files"
	values["TLS_CERT_FILE"] = writeSecret(t, "certificate", "public certificate", 0o644)
	values["TLS_KEY_FILE"] = writeSecret(t, "private-key", "private key", 0o644)
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("publicly readable TLS private key accepted")
	}
	if err := os.Chmod(values["TLS_KEY_FILE"], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseDeployment(values); err != nil {
		t.Fatalf("protected TLS key rejected: %v", err)
	}
}

func TestConditionalDeploymentSecretsUseProtectedFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission semantics")
	}
	values := fixture(t)
	values["TLS_MODE"] = "acme"
	values["ACME_EMAIL"] = "operator@example.com"
	values["ACME_DNS_PROVIDER"] = "example"
	values["ACME_DNS_API_TOKEN_FILE"] = writeSecret(t, "acme-token", "token", 0o644)
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("publicly readable ACME token accepted")
	}
	values = productionFixture(t)
	values["ENABLE_CLOUDFLARE_TUNNEL"] = "true"
	values["TLS_MODE"] = "acme"
	values["ACME_EMAIL"] = "operator@example.com"
	values["ACME_DNS_PROVIDER"] = "cloudflare"
	values["ACME_DNS_API_TOKEN_FILE"] = writeSecret(t, "acme-token", "token", 0o600)
	values["CLOUDFLARE_TUNNEL_ID"] = "tunnel-1"
	values["CLOUDFLARE_TUNNEL_CREDENTIALS_FILE"] = writeSecret(t, "cloudflare", `{"AccountTag":"account","TunnelSecret":"secret","TunnelID":"tunnel-1"}`, 0o644)
	if _, err := ParseDeployment(values); err == nil {
		t.Fatal("publicly readable Cloudflare credentials accepted")
	}
}

func TestSessionAndRedisSecretFormats(t *testing.T) {
	for _, value := range []string{"not-base64", base64.RawStdEncoding.EncodeToString(make([]byte, 32)), base64.StdEncoding.EncodeToString(make([]byte, 31))} {
		if _, err := ReadBase64Key("SESSION_SECRET_FILE", writeSecret(t, "session", value, 0o600), 4096); err == nil {
			t.Errorf("invalid session key %q accepted", value)
		}
	}
	for _, value := range []string{"http://redis:6379", "redis:/missing-host", "redis://redis:6379/#fragment"} {
		if _, _, err := ReadURISecret("REDIS_URL_FILE", writeSecret(t, "redis", value, 0o600), 4096, "redis", "rediss"); err == nil {
			t.Errorf("invalid Redis URI %q accepted", value)
		}
	}
}

func TestDeploymentDatabasePasswordsMustDiffer(t *testing.T) {
	passwordSettings := []string{
		"POSTGRES_PASSWORD_FILE",
		"DATABASE_MIGRATION_PASSWORD_FILE",
		"DATABASE_GATEWAY_PASSWORD_FILE",
		"DATABASE_CONTROL_PLANE_PASSWORD_FILE",
		"DATABASE_WORKER_PASSWORD_FILE",
	}
	for index, name := range passwordSettings {
		other := passwordSettings[(index+1)%len(passwordSettings)]
		t.Run(name+" path", func(t *testing.T) {
			values := fixture(t)
			values[name] = values[other]
			if _, err := ParseDeployment(values); err == nil || !strings.Contains(err.Error(), "distinct files") {
				t.Fatalf("duplicate database password path error = %v", err)
			}
		})
		t.Run(name+" value", func(t *testing.T) {
			values := fixture(t)
			otherSecret, err := ReadSimpleSecret(other, values[other], 4*1024)
			if err != nil {
				t.Fatal(err)
			}
			values[name] = writeSecret(t, "duplicate", otherSecret.Reveal(), 0o600)
			if _, err := ParseDeployment(values); err == nil || !strings.Contains(err.Error(), "distinct passwords") {
				t.Fatalf("duplicate database password value error = %v", err)
			}
		})
	}
}

func TestUnknownApplicationSettingsRejectedAndHostEnvironmentIgnored(t *testing.T) {
	values := fixture(t)
	for name, value := range map[string]string{
		"PATH":                     "/bin",
		"HTTP_PROXY":               "http://proxy.example",
		"HTTPS_PROXY":              "http://proxy.example",
		"OTEL_RESOURCE_ATTRIBUTES": "service.name=external",
		"OTEL_SERVICE_NAME":        "external",
		"REQUESTS_CA_BUNDLE":       "/tmp/ca",
		"SESSION_MANAGER":          "external",
	} {
		values[name] = value
	}
	if _, err := ParseGateway(values); err != nil {
		t.Fatalf("host environment rejected: %v", err)
	}
	for _, name := range []string{"NEXUSRELAY_UNDOCUMENTED", "CSRF_SECRET_FILE", "DATABASE_USER", "SHARED_DATABASE_PASSWORD_FILE"} {
		values := fixture(t)
		values[name] = "value"
		if _, err := ParseGateway(values); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s error = %v", name, err)
		}
	}
}

func TestDependencyReadinessRelationships(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name: "probe timeout before startup timeout",
			values: map[string]string{
				"DEPENDENCY_STARTUP_TIMEOUT": "3s",
				"DEPENDENCY_PROBE_TIMEOUT":   "3s",
			},
			want: "DEPENDENCY_PROBE_TIMEOUT",
		},
		{
			name: "retry minimum before maximum",
			values: map[string]string{
				"DEPENDENCY_RETRY_MIN_DELAY": "3s",
				"DEPENDENCY_RETRY_MAX_DELAY": "2s",
			},
			want: "DEPENDENCY_RETRY_MIN_DELAY",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := fixture(t)
			for name, value := range test.values {
				values[name] = value
			}
			if _, err := ParseGateway(values); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseGateway() error = %v, want %s", err, test.want)
			}
		})
	}
}

func TestProcessClassificationsMatchAcceptedViews(t *testing.T) {
	values := map[string]string{}
	for name := range inventory {
		values[name] = "classified"
	}
	for name, class := range inventory {
		for _, process := range []Process{ProcessGateway, ProcessControlPlane, ProcessWorker} {
			view := ValuesForProcess(values, process)
			_, present := view[name]
			want := false
			for _, consumer := range class.Consumers {
				want = want || consumer == process
			}
			if present != want {
				t.Errorf("%s presence for %s = %v, want %v", name, process, present, want)
			}
		}
	}
	if _, ok := ValuesForProcess(map[string]string{"ADMIN_ORIGINS": "http://localhost:8080"}, ProcessGateway)["ADMIN_ORIGINS"]; ok {
		t.Fatal("gateway received ADMIN_ORIGINS")
	}
	if _, ok := ValuesForProcess(map[string]string{"ADMIN_ORIGINS": "http://localhost:8080"}, ProcessWorker)["ADMIN_ORIGINS"]; ok {
		t.Fatal("worker received ADMIN_ORIGINS")
	}
	databaseCredentials := map[Process][]string{
		ProcessGateway:      {"DATABASE_GATEWAY_USER", "DATABASE_GATEWAY_PASSWORD_FILE"},
		ProcessControlPlane: {"DATABASE_CONTROL_PLANE_USER", "DATABASE_CONTROL_PLANE_PASSWORD_FILE"},
		ProcessWorker:       {"DATABASE_WORKER_USER", "DATABASE_WORKER_PASSWORD_FILE"},
	}
	allCredentials := []string{"POSTGRES_USER", "POSTGRES_PASSWORD_FILE", "DATABASE_MIGRATION_USER", "DATABASE_MIGRATION_PASSWORD_FILE", "DATABASE_GATEWAY_USER", "DATABASE_GATEWAY_PASSWORD_FILE", "DATABASE_CONTROL_PLANE_USER", "DATABASE_CONTROL_PLANE_PASSWORD_FILE", "DATABASE_WORKER_USER", "DATABASE_WORKER_PASSWORD_FILE"}
	for process, expected := range databaseCredentials {
		view := ValuesForProcess(values, process)
		for _, name := range allCredentials {
			want := slices.Contains(expected, name)
			_, got := view[name]
			if got != want {
				t.Errorf("%s credential %s presence = %v, want %v", process, name, got, want)
			}
		}
	}
	deploymentView := ValuesForProcess(values, ProcessDeployment)
	for _, name := range allCredentials {
		if _, ok := deploymentView[name]; !ok {
			t.Errorf("deployment missing credential %s", name)
		}
	}
	for _, name := range []string{"POSTGRES_DB", "DATABASE_NAME", "DATABASE_SSLMODE"} {
		if _, ok := deploymentView[name]; !ok {
			t.Errorf("deployment missing database invariant %s", name)
		}
	}
	for _, name := range []string{"DATABASE_HOST", "DATABASE_PORT", "DATABASE_MIN_CONNECTIONS_GATEWAY", "DATABASE_MAX_CONNECTIONS_GATEWAY", "DATABASE_STATEMENT_TIMEOUT", "DATABASE_TRANSACTION_TIMEOUT"} {
		if _, ok := deploymentView[name]; ok {
			t.Errorf("deployment unexpectedly received runtime database setting %s", name)
		}
	}
	for _, name := range reservedTailscaleSettings {
		if len(inventory[name].Consumers) != 0 {
			t.Errorf("reserved setting %s has an active consumer", name)
		}
		for _, process := range []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap, ProcessDeployment} {
			if _, ok := ValuesForProcess(values, process)[name]; ok {
				t.Errorf("%s unexpectedly received reserved setting %s", process, name)
			}
		}
	}
}

func TestDeploymentRedisPasswordMatchesClientURL(t *testing.T) {
	values := fixture(t)
	if _, err := ParseDeployment(values); err != nil {
		t.Fatalf("matching Redis credentials rejected: %v", err)
	}
	values["REDIS_PASSWORD_FILE"] = writeSecret(t, "other-redis-password", "different_redis_password_0123456789", 0o600)
	if _, err := ParseDeployment(values); err == nil || !strings.Contains(err.Error(), "REDIS_PASSWORD_FILE") {
		t.Fatalf("mismatched Redis credentials error = %v", err)
	}
	values = fixture(t)
	values["REDIS_URL_FILE"] = writeSecret(t, "named-redis-user", "redis://other:redis_password_0123456789abcdefXYZ@redis:6379/0", 0o600)
	if _, err := ParseDeployment(values); err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("unsupported Redis username error = %v", err)
	}
}

func TestReadSecretFileContract(t *testing.T) {
	for _, test := range []struct {
		value   string
		maximum int64
		ok      bool
	}{{"secret", 16, true}, {"secret\n", 16, true}, {"secret\r\n", 16, true}, {"", 16, false}, {" secret", 16, false}, {"secret\n\n", 16, false}, {"secret", 5, false}} {
		path := writeSecret(t, "secret", test.value, 0o600)
		secret, err := ReadSecretFile("TEST_SECRET_FILE", path, test.maximum)
		if test.ok && (err != nil || secret.Reveal() != "secret") {
			t.Errorf("value %q: secret=%q err=%v", test.value, secret.Reveal(), err)
		}
		if !test.ok && err == nil {
			t.Errorf("value %q unexpectedly accepted", test.value)
		}
	}
}

func fixture(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	return map[string]string{
		"POSTGRES_PASSWORD_FILE":               writeAt(t, dir, "cluster-db", "cluster-password", 0o600),
		"DATABASE_MIGRATION_PASSWORD_FILE":     writeAt(t, dir, "migration-db", "migration-password", 0o600),
		"DATABASE_GATEWAY_PASSWORD_FILE":       writeAt(t, dir, "gateway-db", "gateway-password", 0o600),
		"DATABASE_CONTROL_PLANE_PASSWORD_FILE": writeAt(t, dir, "control-db", "control-password", 0o600),
		"DATABASE_WORKER_PASSWORD_FILE":        writeAt(t, dir, "worker-db", "worker-password", 0o600),
		"REDIS_URL_FILE":                       writeAt(t, dir, "redis", "redis://:redis_password_0123456789abcdefXYZ@redis:6379/0", 0o600),
		"REDIS_PASSWORD_FILE":                  writeAt(t, dir, "redis-password", "redis_password_0123456789abcdefXYZ", 0o600),
		"MASTER_KEYRING_FILE":                  writeAt(t, dir, "master", `{"version":1,"expected_epoch":1,"active_key_id":"provider-1","keys":[{"key_id":"provider-1","key":"`+key+`"}]}`, 0o600),
		"API_KEY_PEPPER_RING_FILE":             writeAt(t, dir, "pepper", ringJSON("pepper-1", key), 0o600),
		"SESSION_SECRET_FILE":                  writeAt(t, dir, "session", key, 0o600),
		"CSRF_SECRET_RING_FILE":                writeAt(t, dir, "csrf", ringJSON("csrf-1", key), 0o600),
	}
}

func productionFixture(t *testing.T) map[string]string {
	values := fixture(t)
	values["NEXUSRELAY_ENV"] = "production"
	values["PUBLIC_API_BASE_URL"] = "https://api.example.com/v1"
	values["PUBLIC_API_HOST"] = "api.example.com"
	values["ADMIN_BASE_URL"] = "https://admin.example.com"
	values["ADMIN_HOST"] = "admin.example.com"
	values["ADMIN_ORIGINS"] = "https://admin.example.com"
	values["SESSION_COOKIE_NAME"] = "__Host-nexusrelay_session"
	values["DATABASE_SSLMODE"] = "require"
	return values
}

func ringJSON(id, key string) string {
	return `{"version":1,"active_key_id":"` + id + `","keys":[{"key_id":"` + id + `","key":"` + key + `"}]}`
}
func clone(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}
func writeSecret(t *testing.T, name, value string, mode os.FileMode) string {
	return writeAt(t, t.TempDir(), name, value, mode)
}
func writeAt(t *testing.T, directory, name, value string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(value), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}
func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
