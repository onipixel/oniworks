package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Load reads a config file (YAML or TOML) and merges it into cfg.
// The format is inferred from the file extension.
func Load(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: cannot read %q: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return loadYAML(cfg, data)
	case ".toml":
		return loadTOML(cfg, data)
	default:
		return fmt.Errorf("config: unsupported file format %q (use .yaml, .yml, or .toml)", ext)
	}
}

// LoadDir reads all YAML/TOML files in dir and merges them using the filename
// (without extension) as the top-level key. E.g. config/database.yaml → "database.*".
func LoadDir(cfg *Config, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("config: cannot read directory %q: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" && ext != ".toml" {
			continue
		}
		key := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		path := filepath.Join(dir, e.Name())

		sub := New()
		if err := Load(sub, path); err != nil {
			return err
		}
		cfg.Set(key, sub.All())
	}
	return nil
}

func loadYAML(cfg *Config, data []byte) error {
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("config: YAML parse error: %w", err)
	}
	if m == nil {
		return nil
	}
	normalized, ok := normalizeYAMLMap(m).(map[string]any)
	if !ok {
		return fmt.Errorf("config: YAML root must be a mapping")
	}
	cfg.Merge(normalized)
	return nil
}

func loadTOML(cfg *Config, data []byte) error {
	var m map[string]any
	if _, err := toml.Decode(string(data), &m); err != nil {
		return fmt.Errorf("config: TOML parse error: %w", err)
	}
	if m == nil {
		return nil
	}
	cfg.Merge(m)
	return nil
}

// normalizeYAMLMap converts map[any]any (from YAML unmarshal) to map[string]any recursively.
func normalizeYAMLMap(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[k] = normalizeYAMLMap(vv)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[fmt.Sprintf("%v", k)] = normalizeYAMLMap(vv)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = normalizeYAMLMap(vv)
		}
		return out
	default:
		return val
	}
}
