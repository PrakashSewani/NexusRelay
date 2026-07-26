package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maximumEnvironmentFileBytes = 1024 * 1024

var environmentNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// ReadEnvironmentFile parses the explicit deployment inventory input. It does
// not perform interpolation, quoting, export syntax, or ambient-env merging.
func ReadEnvironmentFile(path string) (map[string]string, error) {
	if path == "" {
		return nil, fmt.Errorf("--env-file is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read environment file: unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("read environment file: must be a regular file")
	}
	return parseEnvironmentFile(io.LimitReader(file, maximumEnvironmentFileBytes+1))
}

func parseEnvironmentFile(reader io.Reader) (map[string]string, error) {
	limited, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read environment file: unreadable")
	}
	if len(limited) > maximumEnvironmentFileBytes {
		return nil, fmt.Errorf("read environment file: exceeds %d bytes", maximumEnvironmentFileBytes)
	}
	if !utf8.Valid(limited) {
		return nil, fmt.Errorf("read environment file: must contain valid UTF-8")
	}
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(limited)))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		for _, character := range line {
			if character == 0 || unicode.IsControl(character) {
				return nil, fmt.Errorf("environment file line %d: contains a control character", lineNumber)
			}
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			return nil, fmt.Errorf("environment file line %d: export syntax is unsupported", lineNumber)
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("environment file line %d: expected KEY=VALUE", lineNumber)
		}
		if !environmentNamePattern.MatchString(name) {
			return nil, fmt.Errorf("environment file line %d: invalid key %q", lineNumber, name)
		}
		if _, known := inventory[name]; !known {
			return nil, fmt.Errorf("environment file line %d: unknown key %s", lineNumber, name)
		}
		if _, duplicate := values[name]; duplicate {
			return nil, fmt.Errorf("environment file line %d: duplicate key %s", lineNumber, name)
		}
		values[name] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read environment file: malformed input")
	}
	return values, nil
}

func ValuesForProcess(values map[string]string, process Process) map[string]string {
	filtered := make(map[string]string)
	for name, value := range values {
		class, known := inventory[name]
		if !known {
			continue
		}
		for _, consumer := range class.Consumers {
			if consumer == process {
				filtered[name] = value
				break
			}
		}
	}
	return filtered
}

func ValidateComplete(values map[string]string) error {
	if _, err := ParseDeployment(ValuesForProcess(values, ProcessDeployment)); err != nil {
		return err
	}
	if _, err := ParseGateway(ValuesForProcess(values, ProcessGateway)); err != nil {
		return err
	}
	if _, err := ParseControlPlane(ValuesForProcess(values, ProcessControlPlane)); err != nil {
		return err
	}
	if _, err := ParseWorker(ValuesForProcess(values, ProcessWorker)); err != nil {
		return err
	}
	return nil
}
