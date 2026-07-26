package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

const keyRingMaximumBytes = 64 * 1024

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Secret struct{ value string }

func (Secret) String() string   { return "[REDACTED]" }
func (Secret) GoString() string { return "config.Secret{[REDACTED]}" }
func (s Secret) Reveal() string { return s.value }

type KeyRing struct {
	activeKeyID string
	keys        map[string][32]byte
}

func (KeyRing) String() string        { return "[REDACTED KEY RING]" }
func (KeyRing) GoString() string      { return "config.KeyRing{[REDACTED]}" }
func (r KeyRing) ActiveKeyID() string { return r.activeKeyID }
func (r KeyRing) KeyCount() int       { return len(r.keys) }

type ProviderKeyRing struct {
	KeyRing
	expectedEpoch int64
}

func (ProviderKeyRing) String() string         { return "[REDACTED PROVIDER KEY RING]" }
func (ProviderKeyRing) GoString() string       { return "config.ProviderKeyRing{[REDACTED]}" }
func (r ProviderKeyRing) ExpectedEpoch() int64 { return r.expectedEpoch }

type CloudflareCredentials struct {
	AccountTag   string `json:"AccountTag"`
	TunnelSecret string `json:"TunnelSecret"`
	TunnelID     string `json:"TunnelID"`
}

func ReadSecretFile(setting, path string, maximumBytes int64) (Secret, error) {
	contents, err := readSecretBytes(setting, path, maximumBytes)
	if err != nil {
		return Secret{}, err
	}
	return Secret{value: string(contents)}, nil
}

func ReadSimpleSecret(setting, path string, maximumBytes int64) (Secret, error) {
	secret, err := ReadSecretFile(setting, path, maximumBytes)
	if err != nil {
		return Secret{}, err
	}
	for _, character := range secret.value {
		if unicode.IsControl(character) {
			return Secret{}, fmt.Errorf("read %s: secret contains a control character", setting)
		}
	}
	return secret, nil
}

func ReadURISecret(setting, path string, maximumBytes int64, schemes ...string) (Secret, *url.URL, error) {
	secret, err := ReadSimpleSecret(setting, path, maximumBytes)
	if err != nil {
		return Secret{}, nil, err
	}
	parsed, err := url.Parse(secret.value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return Secret{}, nil, fmt.Errorf("read %s: secret must contain one absolute URI", setting)
	}
	allowed := false
	for _, scheme := range schemes {
		allowed = allowed || parsed.Scheme == scheme
	}
	if !allowed {
		return Secret{}, nil, fmt.Errorf("read %s: URI scheme is not allowed", setting)
	}
	return secret, parsed, nil
}

func ReadBase64Key(setting, path string, maximumBytes int64) (Secret, error) {
	secret, err := ReadSimpleSecret(setting, path, maximumBytes)
	if err != nil {
		return Secret{}, err
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(secret.value)
	if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != secret.value {
		return Secret{}, fmt.Errorf("read %s: key must be canonical padded base64 for exactly 32 bytes", setting)
	}
	return secret, nil
}

func ReadKeyRing(setting, path string) (KeyRing, error) {
	contents, err := readSecretBytes(setting, path, keyRingMaximumBytes)
	if err != nil {
		return KeyRing{}, err
	}
	var wire struct {
		Version     int    `json:"version"`
		ActiveKeyID string `json:"active_key_id"`
		Keys        []struct {
			KeyID string `json:"key_id"`
			Key   string `json:"key"`
		} `json:"keys"`
	}
	schema := objectSchema{"version": nil, "active_key_id": nil, "keys": arraySchema{objectSchema{"key_id": nil, "key": nil}}}
	if err := strictJSON(setting, contents, &wire, schema); err != nil {
		return KeyRing{}, err
	}
	return validateRing(setting, wire.Version, wire.ActiveKeyID, wire.Keys)
}

func ReadProviderKeyRing(path string) (ProviderKeyRing, error) {
	const setting = "MASTER_KEYRING_FILE"
	contents, err := readSecretBytes(setting, path, keyRingMaximumBytes)
	if err != nil {
		return ProviderKeyRing{}, err
	}
	var wire struct {
		Version       int    `json:"version"`
		ExpectedEpoch int64  `json:"expected_epoch"`
		ActiveKeyID   string `json:"active_key_id"`
		Keys          []struct {
			KeyID string `json:"key_id"`
			Key   string `json:"key"`
		} `json:"keys"`
	}
	schema := objectSchema{"version": nil, "expected_epoch": nil, "active_key_id": nil, "keys": arraySchema{objectSchema{"key_id": nil, "key": nil}}}
	if err := strictJSON(setting, contents, &wire, schema); err != nil {
		return ProviderKeyRing{}, err
	}
	if wire.ExpectedEpoch < 1 {
		return ProviderKeyRing{}, fmt.Errorf("read %s: expected_epoch must be a positive signed 64-bit integer", setting)
	}
	ring, err := validateRing(setting, wire.Version, wire.ActiveKeyID, wire.Keys)
	if err != nil {
		return ProviderKeyRing{}, err
	}
	return ProviderKeyRing{ring, wire.ExpectedEpoch}, nil
}

func ReadCloudflareCredentials(path string) (CloudflareCredentials, error) {
	const setting = "CLOUDFLARE_TUNNEL_CREDENTIALS_FILE"
	contents, err := readSecretBytes(setting, path, keyRingMaximumBytes)
	if err != nil {
		return CloudflareCredentials{}, err
	}
	var credentials CloudflareCredentials
	schema := objectSchema{"AccountTag": nil, "TunnelSecret": nil, "TunnelID": nil}
	if err := strictJSON(setting, contents, &credentials, schema); err != nil {
		return CloudflareCredentials{}, err
	}
	if credentials.AccountTag == "" || credentials.TunnelSecret == "" || credentials.TunnelID == "" {
		return CloudflareCredentials{}, fmt.Errorf("read %s: AccountTag, TunnelSecret, and TunnelID are required", setting)
	}
	return credentials, nil
}

func validateRing(setting string, version int, active string, entries []struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
}) (KeyRing, error) {
	if version != 1 {
		return KeyRing{}, fmt.Errorf("read %s: version must be 1", setting)
	}
	if !keyIDPattern.MatchString(active) {
		return KeyRing{}, fmt.Errorf("read %s: active_key_id is invalid", setting)
	}
	if len(entries) == 0 {
		return KeyRing{}, fmt.Errorf("read %s: keys must not be empty", setting)
	}
	keys := make(map[string][32]byte, len(entries))
	for _, entry := range entries {
		if !keyIDPattern.MatchString(entry.KeyID) {
			return KeyRing{}, fmt.Errorf("read %s: key_id is invalid", setting)
		}
		if _, exists := keys[entry.KeyID]; exists {
			return KeyRing{}, fmt.Errorf("read %s: duplicate key_id", setting)
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(entry.Key)
		if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != entry.Key {
			return KeyRing{}, fmt.Errorf("read %s: key must be canonical padded base64 for exactly 32 bytes", setting)
		}
		var key [32]byte
		copy(key[:], decoded)
		keys[entry.KeyID] = key
	}
	if _, ok := keys[active]; !ok {
		return KeyRing{}, fmt.Errorf("read %s: active_key_id must name one key", setting)
	}
	return KeyRing{active, keys}, nil
}

func validateProtectedFile(setting, path string) error {
	file, err := openProtectedFile(setting, path)
	if err != nil {
		return err
	}
	return file.Close()
}

func openProtectedFile(setting, path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("%s must name a secret file", setting)
	}
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: secret file is unavailable", setting)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("read %s: secret file must not be a symbolic link", setting)
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("read %s: secret file must be a regular file", setting)
	}
	if runtime.GOOS != "windows" && linkInfo.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("read %s: secret file must not be group/world accessible", setting)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: secret file is unavailable", setting)
	}
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(linkInfo, openedInfo) {
		file.Close()
		return nil, fmt.Errorf("read %s: secret file changed while opening", setting)
	}
	return file, nil
}

func readSecretBytes(setting, path string, maximumBytes int64) ([]byte, error) {
	if setting == "" {
		return nil, fmt.Errorf("secret setting name is required")
	}
	if maximumBytes <= 0 {
		return nil, fmt.Errorf("%s has an invalid maximum size", setting)
	}
	file, err := openProtectedFile(setting, path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: secret file could not be read", setting)
	}
	if int64(len(contents)) > maximumBytes {
		return nil, fmt.Errorf("read %s: secret file exceeds %d bytes", setting, maximumBytes)
	}
	if !utf8.Valid(contents) {
		return nil, fmt.Errorf("read %s: secret file must contain valid UTF-8", setting)
	}
	if bytes.HasSuffix(contents, []byte("\r\n")) {
		contents = contents[:len(contents)-2]
	} else if bytes.HasSuffix(contents, []byte("\n")) {
		contents = contents[:len(contents)-1]
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("read %s: secret file must not be empty", setting)
	}
	if strings.TrimSpace(string(contents)) != string(contents) {
		return nil, fmt.Errorf("read %s: secret file contains unintended surrounding whitespace", setting)
	}
	return contents, nil
}

type jsonSchema interface{}
type objectSchema map[string]jsonSchema
type arraySchema struct{ element jsonSchema }

func strictJSON(setting string, contents []byte, destination any, schema jsonSchema) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := scanJSONValue(decoder, schema); err != nil {
		return fmt.Errorf("read %s: invalid JSON: %v", setting, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("read %s: trailing JSON value", setting)
	}
	decoder = json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("read %s: invalid JSON", setting)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("read %s: trailing JSON value", setting)
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, schema jsonSchema) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		fields, ok := schema.(objectSchema)
		if !ok {
			return fmt.Errorf("unexpected object")
		}
		seen := map[string]bool{}
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := nameToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if seen[name] {
				return fmt.Errorf("duplicate field %q", name)
			}
			child, allowed := fields[name]
			if !allowed {
				return fmt.Errorf("unknown field %q", name)
			}
			seen[name] = true
			if err := scanJSONValue(decoder, child); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("unterminated object")
		}
	case '[':
		items, ok := schema.(arraySchema)
		if !ok {
			return fmt.Errorf("unexpected array")
		}
		for decoder.More() {
			if err := scanJSONValue(decoder, items.element); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("unterminated array")
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	return nil
}
