package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const DevSecretDirectoryName = ".local-secrets"

var devSecretFiles = []string{
	"api_key_pepper_ring",
	"csrf_secret_ring",
	"postgres_cluster_admin_password",
	"postgres_control_plane_password",
	"postgres_gateway_password",
	"postgres_migration_password",
	"postgres_worker_password",
	"provider_master_keyring",
	"redis_password",
	"redis_url",
	"session_secret",
}

var deploymentSecretFiles = map[string]string{
	"POSTGRES_PASSWORD_FILE":               "postgres_cluster_admin_password",
	"DATABASE_MIGRATION_PASSWORD_FILE":     "postgres_migration_password",
	"DATABASE_GATEWAY_PASSWORD_FILE":       "postgres_gateway_password",
	"DATABASE_CONTROL_PLANE_PASSWORD_FILE": "postgres_control_plane_password",
	"DATABASE_WORKER_PASSWORD_FILE":        "postgres_worker_password",
	"REDIS_PASSWORD_FILE":                  "redis_password",
	"REDIS_URL_FILE":                       "redis_url",
	"MASTER_KEYRING_FILE":                  "provider_master_keyring",
	"API_KEY_PEPPER_RING_FILE":             "api_key_pepper_ring",
	"SESSION_SECRET_FILE":                  "session_secret",
	"CSRF_SECRET_RING_FILE":                "csrf_secret_ring",
}

func DevSecretFiles() []string {
	return append([]string(nil), devSecretFiles...)
}

func GenerateDevSecrets(directory string) error {
	if directory == "" {
		return fmt.Errorf("output directory is required")
	}
	if _, err := os.Lstat(directory); err == nil {
		if err := ValidateDevSecrets(directory); err != nil {
			return fmt.Errorf("existing development secret inventory is not complete and valid; refusing to overwrite")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect development secret directory: unavailable")
	}

	parent := filepath.Dir(directory)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return fmt.Errorf("output parent directory is unavailable")
	}
	temporary, err := os.MkdirTemp(parent, ".nexusrelay-dev-secrets-*")
	if err != nil {
		return fmt.Errorf("create development secret inventory: unavailable")
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(temporary)
		}
	}()
	if err := os.Chmod(temporary, 0o700); err != nil {
		return fmt.Errorf("protect development secret directory: unavailable")
	}

	passwords := make(map[string]string, 6)
	for _, name := range []string{
		"postgres_cluster_admin_password",
		"postgres_migration_password",
		"postgres_gateway_password",
		"postgres_control_plane_password",
		"postgres_worker_password",
		"redis_password",
	} {
		value, err := randomBase64URL()
		if err != nil {
			return err
		}
		passwords[name] = value
		if err := writeNewSecret(temporary, name, []byte(value)); err != nil {
			return err
		}
	}

	redisURL := (&url.URL{Scheme: "redis", Host: "redis:6379", Path: "/0", User: url.UserPassword("", passwords["redis_password"])}).String()
	if err := writeNewSecret(temporary, "redis_url", []byte(redisURL)); err != nil {
		return err
	}

	providerKey, err := randomBase64()
	if err != nil {
		return err
	}
	providerRing := struct {
		Version       int    `json:"version"`
		ExpectedEpoch int    `json:"expected_epoch"`
		ActiveKeyID   string `json:"active_key_id"`
		Keys          []struct {
			KeyID string `json:"key_id"`
			Key   string `json:"key"`
		} `json:"keys"`
	}{Version: 1, ExpectedEpoch: 1, ActiveKeyID: "provider-dev-1"}
	providerRing.Keys = append(providerRing.Keys, struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}{"provider-dev-1", providerKey})
	if err := writeJSONSecret(temporary, "provider_master_keyring", providerRing); err != nil {
		return err
	}

	for _, ring := range []struct {
		filename string
		keyID    string
	}{
		{"api_key_pepper_ring", "api-pepper-dev-1"},
		{"csrf_secret_ring", "csrf-dev-1"},
	} {
		key, err := randomBase64()
		if err != nil {
			return err
		}
		wire := struct {
			Version     int    `json:"version"`
			ActiveKeyID string `json:"active_key_id"`
			Keys        []struct {
				KeyID string `json:"key_id"`
				Key   string `json:"key"`
			} `json:"keys"`
		}{Version: 1, ActiveKeyID: ring.keyID}
		wire.Keys = append(wire.Keys, struct {
			KeyID string `json:"key_id"`
			Key   string `json:"key"`
		}{ring.keyID, key})
		if err := writeJSONSecret(temporary, ring.filename, wire); err != nil {
			return err
		}
	}

	sessionKey, err := randomBase64()
	if err != nil {
		return err
	}
	if err := writeNewSecret(temporary, "session_secret", []byte(sessionKey)); err != nil {
		return err
	}
	if err := ValidateDevSecrets(temporary); err != nil {
		return fmt.Errorf("validate generated development secret inventory: %w", err)
	}
	if err := os.Mkdir(directory, 0o700); err != nil {
		return fmt.Errorf("publish development secret inventory: destination already exists or is unavailable")
	}
	for _, name := range devSecretFiles {
		if err := os.Rename(filepath.Join(temporary, name), filepath.Join(directory, name)); err != nil {
			return fmt.Errorf("publish development secret inventory: destination is incomplete")
		}
	}
	if err := os.Remove(temporary); err != nil {
		return fmt.Errorf("publish development secret inventory: temporary directory cleanup failed")
	}
	keep = true
	return nil
}

func ValidateDevSecrets(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("development secret root must be a directory, not a symbolic link")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		return fmt.Errorf("development secret root must have mode 0700")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read development secret inventory: unavailable")
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		actual = append(actual, entry.Name())
	}
	sort.Strings(actual)
	if strings.Join(actual, "\x00") != strings.Join(devSecretFiles, "\x00") {
		return fmt.Errorf("development secret inventory must contain exactly %d allowlisted files", len(devSecretFiles))
	}
	for _, name := range devSecretFiles {
		path := filepath.Join(directory, name)
		fileInfo, err := os.Lstat(path)
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("development secret inventory contains an invalid file")
		}
		if runtime.GOOS != "windows" && fileInfo.Mode().Perm() != 0o600 {
			return fmt.Errorf("development secret files must have mode 0600")
		}
	}

	databaseValues := map[string]string{}
	for setting, filename := range deploymentSecretFiles {
		path := filepath.Join(directory, filename)
		switch setting {
		case "POSTGRES_PASSWORD_FILE", "DATABASE_MIGRATION_PASSWORD_FILE", "DATABASE_GATEWAY_PASSWORD_FILE", "DATABASE_CONTROL_PLANE_PASSWORD_FILE", "DATABASE_WORKER_PASSWORD_FILE":
			secret, err := ReadSimpleSecret(setting, path, 4*1024)
			if err != nil {
				return err
			}
			if _, exists := databaseValues[secret.Reveal()]; exists {
				return fmt.Errorf("development database passwords must be distinct")
			}
			databaseValues[secret.Reveal()] = setting
		}
	}
	redisPassword, err := ReadSimpleSecret("REDIS_PASSWORD_FILE", filepath.Join(directory, "redis_password"), 4*1024)
	if err != nil {
		return err
	}
	if len(redisPassword.Reveal()) < 32 || strings.Trim(redisPassword.Reveal(), "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
		return fmt.Errorf("REDIS_PASSWORD_FILE must contain at least 32 base64url characters")
	}
	_, redisURL, err := ReadURISecret("REDIS_URL_FILE", filepath.Join(directory, "redis_url"), 4*1024, "redis")
	if err != nil {
		return err
	}
	urlPassword, ok := redisURL.User.Password()
	if !ok || urlPassword != redisPassword.Reveal() || redisURL.Host != "redis:6379" || redisURL.Path != "/0" {
		return fmt.Errorf("REDIS_URL_FILE does not match the generated development Redis contract")
	}
	if _, err := ReadProviderKeyRing(filepath.Join(directory, "provider_master_keyring")); err != nil {
		return err
	}
	if _, err := ReadKeyRing("API_KEY_PEPPER_RING_FILE", filepath.Join(directory, "api_key_pepper_ring")); err != nil {
		return err
	}
	if _, err := ReadBase64Key("SESSION_SECRET_FILE", filepath.Join(directory, "session_secret"), 4*1024); err != nil {
		return err
	}
	if _, err := ReadKeyRing("CSRF_SECRET_RING_FILE", filepath.Join(directory, "csrf_secret_ring")); err != nil {
		return err
	}
	return nil
}

func ResolveDeploymentSecretRoot(values map[string]string, root string) (map[string]string, error) {
	if err := ValidateDevSecrets(root); err != nil {
		return nil, err
	}
	resolved := make(map[string]string, len(values))
	for name, value := range values {
		resolved[name] = value
	}
	for setting, filename := range deploymentSecretFiles {
		logical := "/run/secrets/" + filename
		value, exists := resolved[setting]
		if !exists {
			value = logical
		}
		if value != logical {
			return nil, fmt.Errorf("%s must equal %s when --secret-root is used", setting, logical)
		}
		resolved[setting] = filepath.Join(root, filename)
	}
	return resolved, nil
}

func ResolveCloudflareSecretRoot(values map[string]string, root string) (map[string]string, error) {
	if root == "" {
		return nil, fmt.Errorf("Cloudflare secret root is required")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("Cloudflare secret root must be a directory")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		return nil, fmt.Errorf("Cloudflare secret root must have mode 0700")
	}
	expected := []string{"acme_dns_api_token", "cloudflare_tunnel_credentials.json"}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != len(expected) {
		return nil, fmt.Errorf("Cloudflare secret inventory differs from the exact allowlist")
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		actual = append(actual, entry.Name())
	}
	sort.Strings(actual)
	for index := range expected {
		if actual[index] != expected[index] {
			return nil, fmt.Errorf("Cloudflare secret inventory differs from the exact allowlist")
		}
	}

	resolved := make(map[string]string, len(values))
	for name, value := range values {
		resolved[name] = value
	}
	for setting, filename := range map[string]string{
		"ACME_DNS_API_TOKEN_FILE":            "acme_dns_api_token",
		"CLOUDFLARE_TUNNEL_CREDENTIALS_FILE": "cloudflare_tunnel_credentials.json",
	} {
		logical := "/run/secrets/" + filename
		if value := resolved[setting]; value != "" && value != logical {
			return nil, fmt.Errorf("%s must equal %s when --cloudflare-secret-root is used", setting, logical)
		}
		path := filepath.Join(root, filename)
		if err := validateProtectedFile(setting, path); err != nil {
			return nil, err
		}
		resolved[setting] = path
	}
	return resolved, nil
}

func randomBase64URL() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate development secret: secure randomness unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func randomBase64() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate development key: secure randomness unavailable")
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func writeJSONSecret(directory, name string, value any) error {
	contents, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode development key ring")
	}
	return writeNewSecret(directory, name, contents)
}

func writeNewSecret(directory, name string, contents []byte) error {
	path := filepath.Join(directory, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create development secret file: unavailable")
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return fmt.Errorf("write development secret file: unavailable")
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close development secret file: unavailable")
	}
	return nil
}
