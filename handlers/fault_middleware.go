package handlers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

// applyFaultMode checks the current simulation mode and injects faults.
// Returns false if the middleware already wrote a response (caller should not proceed).
func (h *Handler) applyFaultMode(w http.ResponseWriter, r *http.Request) bool {
	mode, _ := h.Store.GetMode()

	switch mode {
	case "slow":
		delay := time.Duration(3000+rand.Intn(5000)) * time.Millisecond
		time.Sleep(delay)
		return true

	case "partial":
		return true // handled post-serialization via wrapPartialWriter

	case "pagination-stress":
		return true // handled in handler by overriding paging

	case "rate-limit":
		if rand.Float64() < 0.5 {
			w.Header().Set("Retry-After", "2")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(ocpiutil.Response{
				StatusCode:    ocpiutil.StatusServerError,
				StatusMessage: "Rate limit exceeded — retry after 2 seconds",
				Timestamp:     time.Now().UTC().Format(time.RFC3339),
			})
			return false
		}
		return true

	case "random-500":
		if rand.Float64() < 0.2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ocpiutil.Response{
				StatusCode:    ocpiutil.StatusServerError,
				StatusMessage: "Simulated internal server error",
				Timestamp:     time.Now().UTC().Format(time.RFC3339),
			})
			return false
		}
		return true

	default:
		return true
	}
}

// partialWriter wraps an http.ResponseWriter and truncates the body at a random
// point between 60-80% of the total length for the "partial" fault mode.
type partialWriter struct {
	http.ResponseWriter
	buf []byte
}

func (pw *partialWriter) Write(b []byte) (int, error) {
	pw.buf = append(pw.buf, b...)
	return len(b), nil
}

func (pw *partialWriter) Flush() {
	cutoff := len(pw.buf) * (60 + rand.Intn(20)) / 100
	if cutoff < 1 {
		cutoff = 1
	}
	pw.ResponseWriter.Write(pw.buf[:cutoff])
}

// wrapPartialWriter returns a partialWriter that truncates JSON output.
// Caller must call Flush() on the returned writer after the handler completes.
func wrapPartialWriter(w http.ResponseWriter) *partialWriter {
	return &partialWriter{ResponseWriter: w}
}

// FaultModeMiddleware applies fault mode effects to OCPI endpoints.
func FaultModeMiddleware(h *Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mode, _ := h.Store.GetMode()

			switch mode {
			case "slow":
				delay := time.Duration(3000+rand.Intn(5000)) * time.Millisecond
				time.Sleep(delay)
				next.ServeHTTP(w, r)

			case "partial":
				pw := wrapPartialWriter(w)
				next.ServeHTTP(pw, r)
				pw.Flush()

			case "rate-limit":
				if rand.Float64() < 0.5 {
					w.Header().Set("Retry-After", "2")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(ocpiutil.Response{
						StatusCode:    ocpiutil.StatusServerError,
						StatusMessage: "Rate limit exceeded — retry after 2 seconds",
						Timestamp:     time.Now().UTC().Format(time.RFC3339),
					})
					return
				}
				next.ServeHTTP(w, r)

			case "random-500":
				if rand.Float64() < 0.2 {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(ocpiutil.Response{
						StatusCode:    ocpiutil.StatusServerError,
						StatusMessage: "Simulated internal server error",
						Timestamp:     time.Now().UTC().Format(time.RFC3339),
					})
					return
				}
				next.ServeHTTP(w, r)

			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
