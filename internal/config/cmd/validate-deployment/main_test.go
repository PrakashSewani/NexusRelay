package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAcceptsCompleteSafeFixture(t *testing.T) {
	environmentFile := writeCompleteEnvironment(t, nil)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--env-file", environmentFile}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v, stderr = %q", err, stderr.String())
	}
	if stdout.String() != "deployment configuration valid\n" || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunRequiresEnvironmentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(nil, &stdout, &stderr); err == nil {
		t.Fatal("run() unexpectedly succeeded")
	}
	if !strings.Contains(stderr.String(), "--env-file is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsCompleteConfigurationFailures(t *testing.T) {
	tests := []struct {
		name     string
		override map[string]string
	}{
		{name: "gateway body", override: map[string]string{"REQUEST_BODY_MAX_BYTES": "0"}},
		{name: "session ttl", override: map[string]string{"SESSION_IDLE_TTL": "0s"}},
		{name: "redis", override: map[string]string{"REDIS_URL_FILE": "missing"}},
		{name: "health", override: map[string]string{"HEALTH_CONSECUTIVE_FAILURES": "0"}},
		{name: "retention", override: map[string]string{"REQUEST_RETENTION_DAYS": "0"}},
		{name: "tls", override: map[string]string{"TLS_MODE": "files", "TLS_KEY_FILE": "missing"}},
		{name: "cloudflare", override: map[string]string{"ENABLE_CLOUDFLARE_TUNNEL": "true"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environmentFile := writeCompleteEnvironment(t, test.override)
			var stdout, stderr bytes.Buffer
			if err := run([]string{"--env-file", environmentFile}, &stdout, &stderr); err == nil {
				t.Fatal("run() unexpectedly succeeded")
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "deployment configuration invalid:") {
				t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
			}
			for _, secret := range []string{"cluster-password", "gateway-password", "redis-password"} {
				if strings.Contains(stderr.String(), secret) {
					t.Fatalf("stderr disclosed secret: %q", stderr.String())
				}
			}
		})
	}
}

func TestRunRejectsPasswordSeparation(t *testing.T) {
	directory := t.TempDir()
	shared := writeProtected(t, directory, "shared-password", "same-password")
	environmentFile := writeCompleteEnvironment(t, map[string]string{
		"DATABASE_GATEWAY_PASSWORD_FILE": shared,
		"DATABASE_WORKER_PASSWORD_FILE":  shared,
	})
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--env-file", environmentFile}, &stdout, &stderr); err == nil {
		t.Fatal("run() unexpectedly succeeded")
	}
}

func TestRunRejectsEveryFixedDatabasePrincipalThroughDeployment(t *testing.T) {
	for name, invalid := range map[string]string{
		"POSTGRES_USER":               "postgres",
		"DATABASE_MIGRATION_USER":     "migration",
		"DATABASE_GATEWAY_USER":       "gateway",
		"DATABASE_CONTROL_PLANE_USER": "control",
		"DATABASE_WORKER_USER":        "worker",
	} {
		t.Run(name, func(t *testing.T) {
			environmentFile := writeCompleteEnvironment(t, map[string]string{name: invalid})
			var stdout, stderr bytes.Buffer
			if err := run([]string{"--env-file", environmentFile}, &stdout, &stderr); err == nil {
				t.Fatal("run() unexpectedly succeeded")
			}
			if !strings.Contains(stderr.String(), name) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func writeCompleteEnvironment(t *testing.T, overrides map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	values := map[string]string{
		"NEXUSRELAY_ENV":                       "development",
		"POSTGRES_PASSWORD_FILE":               writeProtected(t, directory, "cluster-db", "cluster-password"),
		"DATABASE_MIGRATION_PASSWORD_FILE":     writeProtected(t, directory, "migration-db", "migration-password"),
		"DATABASE_GATEWAY_PASSWORD_FILE":       writeProtected(t, directory, "gateway-db", "gateway-password"),
		"DATABASE_CONTROL_PLANE_PASSWORD_FILE": writeProtected(t, directory, "control-db", "control-password"),
		"DATABASE_WORKER_PASSWORD_FILE":        writeProtected(t, directory, "worker-db", "worker-password"),
		"REDIS_URL_FILE":                       writeProtected(t, directory, "redis", "redis://:redis-password@redis:6379/0"),
		"MASTER_KEYRING_FILE":                  writeProtected(t, directory, "master", `{"version":1,"expected_epoch":1,"active_key_id":"provider-1","keys":[{"key_id":"provider-1","key":"`+key+`"}]}`),
		"API_KEY_PEPPER_RING_FILE":             writeProtected(t, directory, "pepper", ringJSON("pepper-1", key)),
		"SESSION_SECRET_FILE":                  writeProtected(t, directory, "session", key),
		"CSRF_SECRET_RING_FILE":                writeProtected(t, directory, "csrf", ringJSON("csrf-1", key)),
		"ENABLE_CLOUDFLARE_TUNNEL":             "false",
		"ENABLE_TAILSCALE_PRIVATE_ADMIN":       "false",
		"TLS_MODE":                             "disabled",
	}
	for name, value := range overrides {
		if value == "missing" {
			value = filepath.Join(directory, "missing")
		}
		values[name] = value
	}
	var contents strings.Builder
	for name, value := range values {
		contents.WriteString(name + "=" + value + "\n")
	}
	path := filepath.Join(directory, "deployment.env")
	if err := os.WriteFile(path, []byte(contents.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeProtected(t *testing.T, directory, name, contents string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ringJSON(id, key string) string {
	return `{"version":1,"active_key_id":"` + id + `","keys":[{"key_id":"` + id + `","key":"` + key + `"}]}`
}
