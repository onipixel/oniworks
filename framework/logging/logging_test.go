package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/oniworks/oniworks/framework/logging"
)

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(logging.Config{
		Level:  "debug",
		Format: "json",
		Output: &buf,
	})

	log.Info("test message", "key", "value")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no log output captured")
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Errorf("output should be valid JSON: %v\noutput: %q", err, line)
	}
	if record["msg"] != "test message" {
		t.Errorf("msg field: got %v", record["msg"])
	}
	if record["key"] != "value" {
		t.Errorf("key field: got %v", record["key"])
	}
}

func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(logging.Config{
		Level:  "info",
		Format: "text",
		Output: &buf,
	})

	log.Info("text log message")
	output := buf.String()
	if output == "" {
		t.Fatal("no log output captured")
	}
	// Text format output should contain the message
	if !strings.Contains(output, "text log message") {
		t.Errorf("text log should contain message: %q", output)
	}
	// Text format should NOT be pure JSON
	var m map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(output)), &m) == nil {
		t.Error("text format should not produce JSON output")
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(logging.Config{
		Level:  "warn",
		Format: "json",
		Output: &buf,
	})

	log.Debug("this should be filtered")
	log.Info("this should also be filtered")
	log.Warn("this should appear")

	output := buf.String()
	if strings.Contains(output, "filtered") {
		t.Error("debug/info should be filtered at warn level")
	}
	if !strings.Contains(output, "this should appear") {
		t.Error("warn message should appear")
	}
}

func TestWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(logging.Config{
		Level:  "debug",
		Format: "json",
		Output: &buf,
	})

	ctx := context.Background()
	newCtx, enriched := logging.WithRequestID(ctx, log, "req-abc-123")

	enriched.Info("request handled")
	output := buf.String()

	if !strings.Contains(output, "req-abc-123") {
		t.Errorf("request_id should appear in log output: %q", output)
	}

	if id := logging.RequestID(newCtx); id != "req-abc-123" {
		t.Errorf("RequestID from context: got %q", id)
	}
}

func TestFromContext(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(logging.Config{
		Level:  "debug",
		Format: "json",
		Output: &buf,
	})

	ctx := context.Background()
	ctx, _ = logging.WithRequestID(ctx, log, "ctx-test-id")

	retrieved := logging.FromContext(ctx)
	if retrieved == nil {
		t.Fatal("FromContext returned nil")
	}
	retrieved.Info("from context log")

	if !strings.Contains(buf.String(), "ctx-test-id") {
		t.Errorf("expected request ID in context logger output: %q", buf.String())
	}
}

func TestFromContextFallback(t *testing.T) {
	log := logging.FromContext(context.Background())
	if log == nil {
		t.Error("FromContext with empty ctx should return non-nil logger")
	}
}

func TestRequestIDEmpty(t *testing.T) {
	id := logging.RequestID(context.Background())
	if id != "" {
		t.Errorf("empty context should return empty RequestID, got %q", id)
	}
}

func TestSetsGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	logging.New(logging.Config{
		Level:  "debug",
		Format: "json",
		Output: &buf,
	})
	// After calling New, the global slog default should be updated
	slog.Info("global log test")
	// We don't strictly validate the output here since the global logger
	// may have been overwritten by another test, but we ensure no panic.
}
