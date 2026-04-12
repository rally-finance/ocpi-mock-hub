package correctness

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

type outboundMetaKey struct{}

type OutboundMeta struct {
	ActionID string
}

func WithOutboundMeta(ctx context.Context, meta OutboundMeta) context.Context {
	return context.WithValue(ctx, outboundMetaKey{}, meta)
}

func outboundMetaFromContext(ctx context.Context) OutboundMeta {
	meta, _ := ctx.Value(outboundMetaKey{}).(OutboundMeta)
	return meta
}

type bodyCaptureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *bodyCaptureWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyCaptureWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func Middleware(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if manager == nil || !strings.HasPrefix(r.URL.Path, "/ocpi/") || !manager.ShouldCaptureInboundRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			var requestBody []byte
			if r.Body != nil {
				requestBody, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(requestBody))
			}

			start := time.Now().UTC()
			recorder := &bodyCaptureWriter{ResponseWriter: w}
			next.ServeHTTP(recorder, r)

			event := TrafficEvent{
				Direction:       "inbound",
				Method:          r.Method,
				URL:             requestURL(r),
				Path:            r.URL.Path,
				RawQuery:        r.URL.RawQuery,
				RequestHeaders:  flattenHeaders(r.Header),
				RequestBody:     string(requestBody),
				ResponseStatus:  recorder.status,
				ResponseHeaders: flattenHeaders(recorder.Header()),
				ResponseBody:    recorder.body.String(),
				DurationMS:      time.Since(start).Milliseconds(),
				StartedAt:       start.Format(time.RFC3339Nano),
			}
			manager.RecordTrafficEvent(event)
		})
	}
}

type captureTransport struct {
	base    http.RoundTripper
	manager *Manager
}

func NewHTTPClient(manager *Manager, base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Timeout:       base.Timeout,
		Jar:           base.Jar,
		CheckRedirect: base.CheckRedirect,
		Transport: &captureTransport{
			base:    transport,
			manager: manager,
		},
	}
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.manager == nil {
		return t.base.RoundTrip(req)
	}

	clone := req.Clone(req.Context())
	var requestBody []byte
	if req.Body != nil {
		requestBody, _ = io.ReadAll(req.Body)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(requestBody))
		clone.Body = io.NopCloser(bytes.NewReader(requestBody))
	}

	start := time.Now().UTC()
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		if t.manager.ShouldCaptureOutboundRequest(clone) {
			t.manager.RecordTrafficEvent(TrafficEvent{
				ActionID:       outboundMetaFromContext(clone.Context()).ActionID,
				Direction:      "outbound",
				Method:         clone.Method,
				URL:            clone.URL.String(),
				Path:           clone.URL.Path,
				RawQuery:       clone.URL.RawQuery,
				RequestHeaders: flattenHeaders(clone.Header),
				RequestBody:    string(requestBody),
				ResponseStatus: 0,
				ResponseBody:   err.Error(),
				DurationMS:     time.Since(start).Milliseconds(),
				StartedAt:      start.Format(time.RFC3339Nano),
			})
		}
		return nil, err
	}

	if t.manager.ShouldCaptureOutboundRequest(clone) {
		var responseBody []byte
		if resp.Body != nil {
			responseBody, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(responseBody))
		}
		t.manager.RecordTrafficEvent(TrafficEvent{
			ActionID:        outboundMetaFromContext(clone.Context()).ActionID,
			Direction:       "outbound",
			Method:          clone.Method,
			URL:             clone.URL.String(),
			Path:            clone.URL.Path,
			RawQuery:        clone.URL.RawQuery,
			RequestHeaders:  flattenHeaders(clone.Header),
			RequestBody:     string(requestBody),
			ResponseStatus:  resp.StatusCode,
			ResponseHeaders: flattenHeaders(resp.Header),
			ResponseBody:    string(responseBody),
			DurationMS:      time.Since(start).Milliseconds(),
			StartedAt:       start.Format(time.RFC3339Nano),
		})
	}

	return resp, nil
}

func flattenHeaders(header http.Header) map[string]string {
	if header == nil {
		return nil
	}
	out := make(map[string]string, len(header))
	for key, values := range header {
		out[strings.ToLower(key)] = strings.Join(values, ", ")
	}
	return out
}

func requestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host + r.URL.RequestURI()
}
