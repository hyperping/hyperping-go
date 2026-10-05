// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

// Package mockserver provides a reusable, faithful mock of the Hyperping REST
// API for use in tests. It enforces the same required-field constraints as the
// real server and records every HTTP interaction for white-box assertions.
//
// Usage:
//
//	srv := mockserver.NewMockServer(t)
//	client := hyperping.NewClient("key", hyperping.WithBaseURL(srv.URL), hyperping.WithMaxRetries(0))
//	// ... test using client ...
//	reqs := srv.Requests()
package mockserver

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	hyperping "github.com/hyperping/hyperping-go"
)

// RecordedRequest captures a single HTTP interaction.
type RecordedRequest struct {
	Method string
	Path   string
	Body   []byte
}

// MockOption configures a Server before it starts.
type MockOption func(*mockServerConfig)

type mockServerConfig struct {
	monitors     []hyperping.Monitor
	incidents    []hyperping.Incident
	healthchecks []hyperping.Healthcheck
	maintenance  []hyperping.Maintenance
	outages      []hyperping.Outage
	statusPages  []hyperping.StatusPage
	apiKey       string
	schemaFile   string // reserved for Phase 2 (GO-07)
}

// WithMonitors seeds the mock with an initial set of monitors.
func WithMonitors(monitors []hyperping.Monitor) MockOption {
	return func(c *mockServerConfig) { c.monitors = monitors }
}

// WithIncidents seeds the mock with an initial set of incidents.
func WithIncidents(incidents []hyperping.Incident) MockOption {
	return func(c *mockServerConfig) { c.incidents = incidents }
}

// WithHealthchecks seeds the mock with an initial set of healthchecks.
func WithHealthchecks(healthchecks []hyperping.Healthcheck) MockOption {
	return func(c *mockServerConfig) { c.healthchecks = healthchecks }
}

// WithMaintenance seeds the mock with initial maintenance windows.
func WithMaintenance(maintenance []hyperping.Maintenance) MockOption {
	return func(c *mockServerConfig) { c.maintenance = maintenance }
}

// WithOutages seeds the mock with an initial set of outages.
func WithOutages(outages []hyperping.Outage) MockOption {
	return func(c *mockServerConfig) { c.outages = outages }
}

// WithStatusPages seeds the mock with an initial set of status pages.
func WithStatusPages(pages []hyperping.StatusPage) MockOption {
	return func(c *mockServerConfig) { c.statusPages = pages }
}

// WithAPIKey sets a specific API key the mock will accept.
// When unset, the mock accepts any non-empty Bearer token.
func WithAPIKey(key string) MockOption {
	return func(c *mockServerConfig) { c.apiKey = key }
}

// WithSchemaFile is reserved for Phase 2 spec-driven validation (blocked on GO-07).
// Currently a no-op.
// TODO(GO-07): replace structural validation with specValidator once openapi.yaml is present.
func WithSchemaFile(path string) MockOption {
	return func(c *mockServerConfig) { c.schemaFile = path }
}

// mockStore holds the in-memory state for one server instance.
// All fields are protected by mu.
type mockStore struct {
	mu           sync.RWMutex
	monitors     map[string]*hyperping.Monitor
	incidents    map[string]*hyperping.Incident
	healthchecks map[string]*hyperping.Healthcheck
	maintenance  map[string]*hyperping.Maintenance
	outages      map[string]*hyperping.Outage
	statusPages  map[string]*hyperping.StatusPage
	subscribers  map[string][]hyperping.StatusPageSubscriber // keyed by status page UUID
	reports      map[string]*hyperping.MonitorReport
}

// requestRecorder logs every HTTP interaction.
type requestRecorder struct {
	mu       sync.Mutex
	requests []RecordedRequest
}

func (r *requestRecorder) record(req RecordedRequest) {
	r.mu.Lock()
	r.requests = append(r.requests, req)
	r.mu.Unlock()
}

func (r *requestRecorder) all() []RecordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RecordedRequest, len(r.requests))
	copy(out, r.requests)
	return out
}

// Server wraps *httptest.Server and exposes the recorded request log.
type Server struct {
	*httptest.Server
	// Requests returns the ordered log of all HTTP interactions since the server
	// started. Safe for concurrent read after t.Cleanup runs.
	Requests func() []RecordedRequest
}

// NewMockServer creates an in-process HTTP server that faithfully mimics the
// Hyperping REST API. The server is automatically closed at the end of the test.
func NewMockServer(t *testing.T, opts ...MockOption) *Server {
	t.Helper()

	cfg := &mockServerConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	store := &mockStore{
		monitors:     make(map[string]*hyperping.Monitor),
		incidents:    make(map[string]*hyperping.Incident),
		healthchecks: make(map[string]*hyperping.Healthcheck),
		maintenance:  make(map[string]*hyperping.Maintenance),
		outages:      make(map[string]*hyperping.Outage),
		statusPages:  make(map[string]*hyperping.StatusPage),
		subscribers:  make(map[string][]hyperping.StatusPageSubscriber),
		reports:      make(map[string]*hyperping.MonitorReport),
	}

	for i := range cfg.monitors {
		m := cfg.monitors[i]
		store.monitors[m.UUID] = &m
	}
	for i := range cfg.incidents {
		inc := cfg.incidents[i]
		store.incidents[inc.UUID] = &inc
	}
	for i := range cfg.healthchecks {
		hc := cfg.healthchecks[i]
		store.healthchecks[hc.UUID] = &hc
	}
	for i := range cfg.maintenance {
		mw := cfg.maintenance[i]
		store.maintenance[mw.UUID] = &mw
	}
	for i := range cfg.outages {
		o := cfg.outages[i]
		store.outages[o.UUID] = &o
	}
	for i := range cfg.statusPages {
		sp := cfg.statusPages[i]
		store.statusPages[sp.UUID] = &sp
	}

	rec := &requestRecorder{}
	mux := buildMux(store)
	handler := authMiddleware(cfg.apiKey, recordMiddleware(rec, mux))

	httpSrv := httptest.NewServer(handler)
	t.Cleanup(httpSrv.Close)

	return &Server{
		Server:   httpSrv,
		Requests: rec.all,
	}
}

// buildMux registers all route handlers and returns the ServeMux.
func buildMux(store *mockStore) *http.ServeMux {
	mux := http.NewServeMux()
	registerMonitorHandlers(mux, store)
	registerIncidentHandlers(mux, store)
	registerHealthcheckHandlers(mux, store)
	registerMaintenanceHandlers(mux, store)
	registerOutageHandlers(mux, store)
	registerStatusPageHandlers(mux, store)
	registerReportHandlers(mux, store)
	return mux
}

// authMiddleware enforces Bearer token authentication on every request.
func authMiddleware(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing api key")
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing api key")
			return
		}
		if apiKey != "" && token != apiKey {
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recordMiddleware captures the method, path, and body of every request.
func recordMiddleware(rec *requestRecorder, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}
		rec.record(RecordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Body:   body,
		})
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// newUUID generates a random UUID-like identifier.
func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
