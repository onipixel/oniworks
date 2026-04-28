// Package config provides environment and file-based configuration with type-safe accessors.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds the merged configuration from all sources (env, YAML, TOML, .env files).
type Config struct {
	mu   sync.RWMutex
	data map[string]any
}

// New returns an empty Config.
func New() *Config {
	return &Config{data: make(map[string]any)}
}

// Set stores a value at the given dot-notation key (e.g. "database.host").
func (c *Config) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	setNested(c.data, strings.Split(key, "."), value)
}

// Merge merges a map of values into the configuration (dot-notation keys).
func (c *Config) Merge(values map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	mergeInto(c.data, values)
}

// Get returns the raw value at key, or nil if not found.
func (c *Config) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return getNested(c.data, strings.Split(key, "."))
}

// GetD returns the value at key, falling back to def if absent.
func (c *Config) GetD(key string, def any) any {
	v := c.Get(key)
	if v == nil {
		return def
	}
	return v
}

// String returns the value at key as a string, falling back to def.
func (c *Config) String(key, def string) string {
	v := c.Get(key)
	if v == nil {
		return def
	}
	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Int returns the value at key as an int, falling back to def.
func (c *Config) Int(key string, def int) int {
	v := c.Get(key)
	if v == nil {
		return def
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		n, err := strconv.Atoi(val)
		if err != nil {
			return def
		}
		return n
	}
	return def
}

// Bool returns the value at key as a bool, falling back to def.
func (c *Config) Bool(key string, def bool) bool {
	v := c.Get(key)
	if v == nil {
		return def
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return def
		}
		return b
	}
	return def
}

// Duration returns the value at key as time.Duration (parsed from string e.g. "5m", "1h").
func (c *Config) Duration(key string, def time.Duration) time.Duration {
	v := c.Get(key)
	if v == nil {
		return def
	}
	switch val := v.(type) {
	case time.Duration:
		return val
	case string:
		d, err := time.ParseDuration(val)
		if err != nil {
			return def
		}
		return d
	case int:
		return time.Duration(val) * time.Second
	case float64:
		return time.Duration(val) * time.Second
	}
	return def
}

// Env returns an env variable by name, falling back to def.
func Env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// EnvRequired returns an env variable or panics if absent.
func EnvRequired(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("config: required environment variable %q is not set", key))
	}
	return v
}

// All returns a shallow copy of the top-level map.
func (c *Config) All() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]any, len(c.data))
	for k, v := range c.data {
		out[k] = v
	}
	return out
}

// --- helpers ---

func setNested(m map[string]any, keys []string, value any) {
	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}
	sub, ok := m[keys[0]].(map[string]any)
	if !ok {
		sub = make(map[string]any)
		m[keys[0]] = sub
	}
	setNested(sub, keys[1:], value)
}

func getNested(m map[string]any, keys []string) any {
	if len(keys) == 0 {
		return nil
	}
	v, ok := m[keys[0]]
	if !ok {
		return nil
	}
	if len(keys) == 1 {
		return v
	}
	sub, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return getNested(sub, keys[1:])
}

func mergeInto(dst, src map[string]any) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]any); ok {
			if dstMap, ok := dst[k].(map[string]any); ok {
				mergeInto(dstMap, srcMap)
				continue
			}
			newMap := make(map[string]any)
			mergeInto(newMap, srcMap)
			dst[k] = newMap
			continue
		}
		dst[k] = v
	}
}
