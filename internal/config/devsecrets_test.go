package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateDevSecretsCreatesExactProtectedInventory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), DevSecretDirectoryName)
	if err := GenerateDevSecrets(directory); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDevSecrets(directory); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(directory, "session_secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := GenerateDevSecrets(directory); err != nil {
		t.Fatalf("complete valid inventory was not idempotent: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(directory, "session_secret"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("idempotent generation changed an existing secret")
	}
}

func TestGenerateDevSecretsRefusesIncompleteChangedOrExtraInventory(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"missing", func(t *testing.T, directory string) {
			t.Helper()
			if err := os.Remove(filepath.Join(directory, "session_secret")); err != nil {
				t.Fatal(err)
			}
		}},
		{"changed", func(t *testing.T, directory string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(directory, "session_secret"), []byte("changed"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"extra", func(t *testing.T, directory string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(directory, "extra"), []byte("extra"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), DevSecretDirectoryName)
			if err := GenerateDevSecrets(directory); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, directory)
			if err := GenerateDevSecrets(directory); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestResolveDeploymentSecretRootPreservesLogicalContract(t *testing.T) {
	directory := filepath.Join(t.TempDir(), DevSecretDirectoryName)
	if err := GenerateDevSecrets(directory); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for setting, filename := range deploymentSecretFiles {
		values[setting] = "/run/secrets/" + filename
	}
	resolved, err := ResolveDeploymentSecretRoot(values, directory)
	if err != nil {
		t.Fatal(err)
	}
	for setting, filename := range deploymentSecretFiles {
		if resolved[setting] != filepath.Join(directory, filename) {
			t.Fatalf("%s = %q", setting, resolved[setting])
		}
	}
	values["SESSION_SECRET_FILE"] = filepath.Join(directory, "session_secret")
	if _, err := ResolveDeploymentSecretRoot(values, directory); err == nil {
		t.Fatal("physical path unexpectedly accepted as a logical deployment path")
	}
}

func TestResolveCloudflareSecretRootPreservesLogicalContract(t *testing.T) {
	directory := filepath.Join(t.TempDir(), ".cloudflare-secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"acme_dns_api_token":                 "token",
		"cloudflare_tunnel_credentials.json": `{"AccountTag":"account","TunnelSecret":"secret","TunnelID":"tunnel-1"}`,
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	values := map[string]string{
		"ACME_DNS_API_TOKEN_FILE":            "/run/secrets/acme_dns_api_token",
		"CLOUDFLARE_TUNNEL_CREDENTIALS_FILE": "/run/secrets/cloudflare_tunnel_credentials.json",
	}
	resolved, err := ResolveCloudflareSecretRoot(values, directory)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["ACME_DNS_API_TOKEN_FILE"] != filepath.Join(directory, "acme_dns_api_token") ||
		resolved["CLOUDFLARE_TUNNEL_CREDENTIALS_FILE"] != filepath.Join(directory, "cloudflare_tunnel_credentials.json") {
		t.Fatalf("unexpected resolved paths: %#v", resolved)
	}
	if err := os.WriteFile(filepath.Join(directory, "extra"), []byte("extra"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveCloudflareSecretRoot(values, directory); err == nil {
		t.Fatal("extra Cloudflare secret accepted")
	}
}
