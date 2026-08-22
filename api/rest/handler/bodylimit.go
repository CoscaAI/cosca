package handler

import (
	"errors"
	"net/http"
)

// Request body size limits (A5 — unbounded request bodies). Every handler
// that decodes a JSON body caps the body with limitBody; reads beyond the
// cap fail with *http.MaxBytesError, which writeDecodeError maps to HTTP 413
// Payload Too Large (fail closed — the request is never fully buffered).
const (
	// bodyLimitSmall is for small JSON bodies (auth, user management, API
	// keys, providers, skills, audit, emergency): 1 MiB.
	bodyLimitSmall = int64(1 << 20)

	// bodyLimitMedium is for bodies that may carry moderate payloads
	// (knowledge search/index, secrets, memory promote): 2 MiB.
	bodyLimitMedium = int64(2 << 20)

	// bodyLimitLarge is for bodies that legitimately carry bulk content
	// (memory store, run execute/stream): 5 MiB.
	bodyLimitLarge = int64(5 << 20)
)

// limitBody caps the request body at maxBytes using http.MaxBytesReader.
// The limit is enforced incrementally: a client streaming more than maxBytes
// is cut off with a *http.MaxBytesError before the whole body is read.
func limitBody(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
}

// writeDecodeError maps a JSON body decode error to the correct status code:
// HTTP 413 Payload Too Large when the body exceeded the limit set by
// limitBody (*http.MaxBytesError), HTTP 400 Bad Request otherwise. It must
// be called before any write to w.
func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
}
