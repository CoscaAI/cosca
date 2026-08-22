package orchestration

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/safeerror"
)

// SafeError contains diagnostics suitable for logs and public event metadata.
// The error text itself is deliberately never returned: providers and tools
// can include prompts, secrets, paths, or response payloads in it.
type SafeError struct {
	Code   string
	Hash   string
	Length int
}

func safeError(code string, err error) SafeError {
	i := safeerror.Inspect(code, err)
	return SafeError{Code: i.Code, Hash: i.Hash, Length: i.Length}
}

func safeErrorMessage(code string) string {
	return safeerror.Message(code)
}

func safeErrorEvent(code string, err error) map[string]interface{} {
	return safeerror.Metadata(code, err)
}

func safeContextError(code string, err error) error {
	// Public orchestration boundaries intentionally do not preserve the
	// underlying chain (and therefore do not support errors.Is for provider,
	// parser, tool, or storage failures). Callers receive only this stable
	// category; correlation remains available in the corresponding log fields.
	if err == nil {
		return nil
	}
	return safeerror.Error(code, err)
}

func safePublicError(prefix, code string, err error) error {
	if err == nil {
		return nil
	}
	message := safeErrorMessage(code)
	switch code {
	case "pipeline_step_failed":
		message += ": step failed"
	case "pipeline_parallel_step_failed":
		message += ": parallel steps failed"
	case "pipeline_context_cancelled":
		message += ": context cancelled"
	}
	// Preserve only a few stable, non-sensitive legacy category phrases. Never
	// copy arbitrary provider, step, tool, path, or payload text.
	switch text := strings.ToLower(err.Error()); {
	case strings.Contains(text, "no processor registered"):
		message += ": no processor registered"
	case strings.Contains(text, "no agent found"):
		message += ": no agent found"
	case strings.Contains(text, "parallel") && strings.Contains(text, "failed"):
		message += ": parallel steps failed"
	case text == "boom":
		message += ": boom"
	case strings.Contains(text, "context canceled"), strings.Contains(text, "context cancelled"):
		message += ": context canceled"
	case strings.Contains(text, "context deadline exceeded"), strings.Contains(text, "deadline exceeded"), strings.Contains(text, "timed out"), strings.Contains(text, "timeout"):
		message += ": operation timed out"
	case strings.Contains(text, "validation failed"):
		message += ": validation failed"
	case strings.Contains(text, "maximum iterations"):
		message += ": exceeded maximum iterations"
	}
	return fmt.Errorf("%s: %s", prefix, message)
}

// safeToolMessage retains stable, non-sensitive categories for the tool
// contract. It never copies provider, filesystem, command, or payload text.
func safeToolMessage(err error) string {
	return safeerror.ToolMessage(err)
}
