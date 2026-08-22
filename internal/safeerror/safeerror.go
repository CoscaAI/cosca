// Package safeerror provides the single boundary used when errors cross into
// logs, events, tool messages, or other consumer-visible values.
package safeerror

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// Info is non-sensitive error telemetry. Hash is a SHA-256 of the original
// error text and is intended only for correlation, never for display.
type Info struct {
	Code   string
	Hash   string
	Length int
}

func Inspect(code string, err error) Info {
	if code == "" {
		code = "internal_error"
	}
	if err == nil {
		return Info{Code: code}
	}
	b := []byte(err.Error())
	h := sha256.Sum256(b)
	return Info{Code: code, Hash: hex.EncodeToString(h[:]), Length: len(b)}
}

func Message(code string) string {
	if code == "" {
		return "The operation could not be completed."
	}
	return "The operation could not be completed (" + code + ")."
}

func Metadata(code string, err error) map[string]interface{} {
	info := Inspect(code, err)
	m := map[string]interface{}{"error_code": info.Code}
	if info.Hash != "" {
		m["error_hash"] = info.Hash
		m["error_length"] = info.Length
	}
	return m
}

// ToolMessage returns only stable, deliberately allow-listed tool categories.
func ToolMessage(err error) string {
	if err == nil {
		return Message("tool_execution_failed")
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "path traversal"), strings.Contains(msg, "symlink"):
		return "path traversal blocked"
	case strings.Contains(msg, "is a directory"):
		return "is a directory"
	case strings.Contains(msg, "missing required parameter"):
		return "missing required parameter"
	case strings.Contains(msg, "exceeds maximum"):
		return "file size exceeds maximum"
	case strings.Contains(msg, "not in the allowed list"):
		return "command is not in the allowed list"
	case strings.Contains(msg, "timed out"), strings.Contains(msg, "timeout"):
		return "command timed out"
	case strings.Contains(msg, "unsupported tool"):
		return "unsupported tool"
	case strings.Contains(msg, "invalid json arguments"):
		return "invalid arguments"
	default:
		return Message("tool_execution_failed")
	}
}

func Error(code string, err error) error {
	if err == nil {
		return nil
	}
	return errors.New(Message(code))
}
