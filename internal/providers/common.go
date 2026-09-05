// Package providers provides shared utilities for all provider implementations.
package providers

import "net/http"

// TruncateString truncates a string to maxLen characters, appending "..." if truncated.
func TruncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// TruncateBody truncates error response bodies for logging.
func TruncateBody(body string) string {
	if len(body) > 200 {
		return body[:200] + "..."
	}
	return body
}

// IsNonRetryable returns true for HTTP status codes that should not be retried.
func IsNonRetryable(statusCode int) bool {
	return statusCode == http.StatusBadRequest ||
		statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden ||
		statusCode == http.StatusNotFound
}

// EstimateTokens estimates the token count for a set of texts.
// Uses a rough heuristic of ~4 characters per token.
func EstimateTokens(texts []string) int {
	total := 0
	for _, t := range texts {
		total += len(t) / 4
	}
	return total
}
