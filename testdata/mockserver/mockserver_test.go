// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	hyperping "github.com/hyperping/hyperping-go"
	"github.com/hyperping/hyperping-go/testdata/mockserver"
)

// helpers

func makeReq(t *testing.T, method, url, token string, body interface{}) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func mustDecode(t *testing.T, resp *http.Response, dst interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// =============================================================================
// Auth
// =============================================================================

func TestMockServer_Auth_MissingHeader(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	resp := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestMockServer_Auth_EmptyBearer(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/v1/monitors", nil)
	req.Header.Set("Authorization", "Bearer ")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestMockServer_Auth_InvalidKey(t *testing.T) {
	srv := mockserver.NewMockServer(t, mockserver.WithAPIKey("secret"))
	resp := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "wrong", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestMockServer_Auth_ValidKey(t *testing.T) {
	srv := mockserver.NewMockServer(t, mockserver.WithAPIKey("secret"))
	resp := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "secret", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Monitor CRUD
// =============================================================================

func TestMockServer_Monitor_List_Empty(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	resp := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "any", nil)
	var monitors []hyperping.Monitor
	mustDecode(t, resp, &monitors)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	if len(monitors) != 0 {
		t.Errorf("want empty slice, got %d items", len(monitors))
	}
}

func TestMockServer_Monitor_List_Seeded(t *testing.T) {
	seeded := []hyperping.Monitor{
		{UUID: "mon_001", Name: "M1", URL: "https://a.com", Protocol: "http"},
		{UUID: "mon_002", Name: "M2", URL: "https://b.com", Protocol: "http"},
	}
	srv := mockserver.NewMockServer(t, mockserver.WithMonitors(seeded))
	resp := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "any", nil)
	var monitors []hyperping.Monitor
	mustDecode(t, resp, &monitors)
	if len(monitors) != 2 {
		t.Fatalf("want 2 monitors, got %d", len(monitors))
	}
	if monitors[0].UUID != "mon_001" {
		t.Errorf("want mon_001, got %q", monitors[0].UUID)
	}
}

func TestMockServer_Monitor_Create_RequiredFields(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	// Missing url and protocol
	body := map[string]string{"name": "only-name"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v1/monitors", "any", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestMockServer_Monitor_Create_Success(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	body := hyperping.CreateMonitorRequest{
		Name:     "test-mon",
		URL:      "https://example.com",
		Protocol: "http",
	}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v1/monitors", "any", body)
	var created hyperping.Monitor
	mustDecode(t, resp, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("want 201, got %d", resp.StatusCode)
	}
	if created.UUID == "" {
		t.Error("expected non-empty UUID in created monitor")
	}
	if created.Name != "test-mon" {
		t.Errorf("want name 'test-mon', got %q", created.Name)
	}

	// GET confirms round-trip
	resp2 := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors/"+created.UUID, "any", nil)
	var fetched hyperping.Monitor
	mustDecode(t, resp2, &fetched)
	if fetched.UUID != created.UUID {
		t.Errorf("round-trip UUID mismatch: %q vs %q", fetched.UUID, created.UUID)
	}
}

func TestMockServer_Monitor_Get_NotFound(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	resp := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors/doesnotexist", "any", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestMockServer_Monitor_Delete(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	// Create one
	body := hyperping.CreateMonitorRequest{Name: "del-me", URL: "https://x.com", Protocol: "http"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v1/monitors", "any", body)
	var created hyperping.Monitor
	mustDecode(t, resp, &created)

	// Delete it
	resp2 := makeReq(t, http.MethodDelete, srv.URL+"/v1/monitors/"+created.UUID, "any", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("want 204, got %d", resp2.StatusCode)
	}

	// Subsequent GET returns 404
	resp3 := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors/"+created.UUID, "any", nil)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 after delete, got %d", resp3.StatusCode)
	}
}

func TestMockServer_Monitor_Pause_Resume(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	body := hyperping.CreateMonitorRequest{Name: "pause-me", URL: "https://y.com", Protocol: "http"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v1/monitors", "any", body)
	var created hyperping.Monitor
	mustDecode(t, resp, &created)

	// Pause
	resp2 := makeReq(t, http.MethodPost, srv.URL+"/v1/monitors/"+created.UUID+"/pause", "any", nil)
	var paused hyperping.Monitor
	mustDecode(t, resp2, &paused)
	if !paused.Paused {
		t.Error("expected Paused=true after pause")
	}

	// Resume
	resp3 := makeReq(t, http.MethodPost, srv.URL+"/v1/monitors/"+created.UUID+"/resume", "any", nil)
	var resumed hyperping.Monitor
	mustDecode(t, resp3, &resumed)
	if resumed.Paused {
		t.Error("expected Paused=false after resume")
	}
}

// =============================================================================
// Incident CRUD
// =============================================================================

func TestMockServer_Incident_Create_RequiredFields(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	// Missing title
	body := map[string]string{"type": "incident"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v3/incidents", "any", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestMockServer_Incident_Create_Success(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	body := hyperping.CreateIncidentRequest{
		Title: hyperping.LocalizedText{En: "Test outage"},
		Type:  "incident",
	}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v3/incidents", "any", body)
	var created hyperping.Incident
	mustDecode(t, resp, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("want 201, got %d", resp.StatusCode)
	}
	if created.UUID == "" {
		t.Error("expected non-empty UUID")
	}

	// GET round-trip
	resp2 := makeReq(t, http.MethodGet, srv.URL+"/v3/incidents/"+created.UUID, "any", nil)
	var fetched hyperping.Incident
	mustDecode(t, resp2, &fetched)
	if fetched.UUID != created.UUID {
		t.Errorf("round-trip mismatch: %q vs %q", fetched.UUID, created.UUID)
	}
}

func TestMockServer_Incident_AddUpdate(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	body := hyperping.CreateIncidentRequest{
		Title: hyperping.LocalizedText{En: "Outage"},
		Type:  "incident",
	}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v3/incidents", "any", body)
	var created hyperping.Incident
	mustDecode(t, resp, &created)

	upd := hyperping.AddIncidentUpdateRequest{
		Text: hyperping.LocalizedText{En: "Investigating"},
		Type: "investigating",
		Date: "2026-01-01T00:00:00Z",
	}
	resp2 := makeReq(t, http.MethodPost, srv.URL+"/v3/incidents/"+created.UUID+"/updates", "any", upd)
	var updated hyperping.Incident
	mustDecode(t, resp2, &updated)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp2.StatusCode)
	}
	if len(updated.Updates) == 0 {
		t.Error("expected at least one update on the incident")
	}
}

// =============================================================================
// StatusPage CRUD
// =============================================================================

func TestMockServer_StatusPage_Create_RequiredFields(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	body := map[string]string{"subdomain": "no-name"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v2/statuspages", "any", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestMockServer_StatusPage_Create_Success(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	body := hyperping.CreateStatusPageRequest{Name: "My Page"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v2/statuspages", "any", body)
	var wrapper struct {
		StatusPage hyperping.StatusPage `json:"statuspage"`
	}
	mustDecode(t, resp, &wrapper)
	created := wrapper.StatusPage
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("want 201, got %d", resp.StatusCode)
	}
	if created.UUID == "" {
		t.Error("expected non-empty UUID")
	}

	// GET round-trip - response is wrapped {"statuspage": {...}}
	resp2 := makeReq(t, http.MethodGet, srv.URL+"/v2/statuspages/"+created.UUID, "any", nil)
	var wrapper2 struct {
		StatusPage hyperping.StatusPage `json:"statuspage"`
	}
	mustDecode(t, resp2, &wrapper2)
	fetched := wrapper2.StatusPage
	if fetched.UUID != created.UUID {
		t.Errorf("round-trip mismatch: %q vs %q", fetched.UUID, created.UUID)
	}

	// Pagination list
	resp3 := makeReq(t, http.MethodGet, srv.URL+"/v2/statuspages", "any", nil)
	var pagResp hyperping.StatusPagePaginatedResponse
	mustDecode(t, resp3, &pagResp)
	if pagResp.Total < 1 {
		t.Errorf("want at least 1 status page in list, got %d", pagResp.Total)
	}
}

func TestMockServer_StatusPage_AddSubscriber_RequiredFields(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	// Create a status page first
	body := hyperping.CreateStatusPageRequest{Name: "Sub Page"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v2/statuspages", "any", body)
	var wrapper struct {
		StatusPage hyperping.StatusPage `json:"statuspage"`
	}
	mustDecode(t, resp, &wrapper)
	sp := wrapper.StatusPage

	// Add subscriber without required fields
	badSub := map[string]string{"type": "email"}
	resp2 := makeReq(t, http.MethodPost, srv.URL+"/v2/statuspages/"+sp.UUID+"/subscribers", "any", badSub)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp2.StatusCode)
	}
}

func TestMockServer_StatusPage_AddSubscriber_Success(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	body := hyperping.CreateStatusPageRequest{Name: "Sub Page2"}
	resp := makeReq(t, http.MethodPost, srv.URL+"/v2/statuspages", "any", body)
	var wrapper struct {
		StatusPage hyperping.StatusPage `json:"statuspage"`
	}
	mustDecode(t, resp, &wrapper)
	sp := wrapper.StatusPage

	email := "test@example.com"
	sub := hyperping.AddSubscriberRequest{Type: "email", Email: &email}
	resp2 := makeReq(t, http.MethodPost, srv.URL+"/v2/statuspages/"+sp.UUID+"/subscribers", "any", sub)
	var subWrapper struct {
		Subscriber hyperping.StatusPageSubscriber `json:"subscriber"`
	}
	mustDecode(t, resp2, &subWrapper)
	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("want 201, got %d", resp2.StatusCode)
	}

	// List subscribers
	resp3 := makeReq(t, http.MethodGet, srv.URL+"/v2/statuspages/"+sp.UUID+"/subscribers", "any", nil)
	var pagResp hyperping.SubscriberPaginatedResponse
	mustDecode(t, resp3, &pagResp)
	if len(pagResp.Subscribers) != 1 {
		t.Errorf("want 1 subscriber, got %d", len(pagResp.Subscribers))
	}
}

// =============================================================================
// Production client compatibility
// =============================================================================

func TestMockServer_ClientLibraryCompat_Monitors(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	client := hyperping.NewClient("test_key",
		hyperping.WithBaseURL(srv.URL),
		hyperping.WithMaxRetries(0),
	)
	ctx := context.Background()

	// List (empty)
	monitors, err := client.ListMonitors(ctx)
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	if monitors == nil {
		t.Error("expected non-nil slice from ListMonitors")
	}

	// Create
	created, err := client.CreateMonitor(ctx, hyperping.CreateMonitorRequest{
		Name:     "compat-mon",
		URL:      "https://compat.example.com",
		Protocol: "http",
	})
	if err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	if created.UUID == "" {
		t.Error("expected non-empty UUID from CreateMonitor")
	}

	// Get
	got, err := client.GetMonitor(ctx, created.UUID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if got.UUID != created.UUID {
		t.Errorf("GetMonitor UUID mismatch: %q vs %q", got.UUID, created.UUID)
	}

	// Delete
	if err := client.DeleteMonitor(ctx, created.UUID); err != nil {
		t.Fatalf("DeleteMonitor: %v", err)
	}
}

func TestMockServer_ClientLibraryCompat_Incidents(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	client := hyperping.NewClient("test_key",
		hyperping.WithBaseURL(srv.URL),
		hyperping.WithMaxRetries(0),
	)
	ctx := context.Background()

	// List (empty)
	incidents, err := client.ListIncidents(ctx)
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if incidents == nil {
		t.Error("expected non-nil slice from ListIncidents")
	}

	// Create
	created, err := client.CreateIncident(ctx, hyperping.CreateIncidentRequest{
		Title: hyperping.LocalizedText{En: "Test incident"},
		Type:  "incident",
	})
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if created.UUID == "" {
		t.Error("expected non-empty UUID from CreateIncident")
	}

	// Get
	got, err := client.GetIncident(ctx, created.UUID)
	if err != nil {
		t.Fatalf("GetIncident: %v", err)
	}
	if got.UUID != created.UUID {
		t.Errorf("GetIncident UUID mismatch: %q vs %q", got.UUID, created.UUID)
	}
}

// =============================================================================
// Request recording
// =============================================================================

func TestMockServer_RequestLog_RecordsAll(t *testing.T) {
	srv := mockserver.NewMockServer(t)

	makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "any", nil).Body.Close()
	makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "any", nil).Body.Close()
	makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "any", nil).Body.Close()

	reqs := srv.Requests()
	if len(reqs) != 3 {
		t.Errorf("want 3 recorded requests, got %d", len(reqs))
	}
}

func TestMockServer_RequestLog_BodyCaptured(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	payload := hyperping.CreateMonitorRequest{
		Name:     "body-check",
		URL:      "https://body.example.com",
		Protocol: "http",
	}
	makeReq(t, http.MethodPost, srv.URL+"/v1/monitors", "any", payload).Body.Close()

	reqs := srv.Requests()
	if len(reqs) != 1 {
		t.Fatalf("want 1 recorded request, got %d", len(reqs))
	}
	if !strings.Contains(string(reqs[0].Body), "body-check") {
		t.Errorf("expected body to contain 'body-check', got: %s", string(reqs[0].Body))
	}
}

// =============================================================================
// Phase 2 stub
// =============================================================================

func TestMockServer_WithSchemaFile_NoOp(t *testing.T) {
	// WithSchemaFile is reserved for Phase 2 (GO-07); it must not panic.
	srv := mockserver.NewMockServer(t, mockserver.WithSchemaFile("nonexistent.yaml"))
	resp := makeReq(t, http.MethodGet, srv.URL+"/v1/monitors", "any", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 even with no-op schema file, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Extra: 404 for unknown path
// =============================================================================

func TestMockServer_UnknownPath_404(t *testing.T) {
	srv := mockserver.NewMockServer(t)
	resp := makeReq(t, http.MethodGet, srv.URL+"/v9/unknown", "any", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for unknown path, got %d", resp.StatusCode)
	}
}

// suppress unused import warning during initial skeleton
var _ = fmt.Sprintf
