package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadEnv parses a .env file and sets each key=value pair as an environment variable.
// Existing OS env variables are NOT overwritten (use LoadEnvOverride for that).
func LoadEnv(path string) error {
	return parseEnvFile(path, false)
}

// LoadEnvOverride parses a .env file and overwrites existing OS env variables.
func LoadEnvOverride(path string) error {
	return parseEnvFile(path, true)
}

func parseEnvFile(path string, override bool) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // silently skip missing .env
		}
		return fmt.Errorf("config: cannot open %q: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, err := parseLine(line)
		if err != nil {
			return fmt.Errorf("config: .env line %d: %w", lineNum, err)
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists && !override {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("config: cannot set env %q: %w", key, err)
		}
	}
	return scanner.Err()
}

func parseLine(line string) (key, value string, err error) {
	// Strip optional "export " prefix
	line = strings.TrimPrefix(line, "export ")
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", fmt.Errorf("invalid line %q: missing '='", line)
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	// Strip surrounding quotes and unescape
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
			value = strings.ReplaceAll(value, `\n`, "\n")
			value = strings.ReplaceAll(value, `\t`, "\t")
		}
	}

	// Strip inline comments (only outside quotes)
	if !strings.HasPrefix(value, `"`) && !strings.HasPrefix(value, `'`) {
		if idx := strings.IndexByte(value, '#'); idx > 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}

	return key, value, nil
}
