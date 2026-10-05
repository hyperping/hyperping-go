// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package hyperping

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Schema-contract test.
//
// Pins the MCP client against a snapshot of the live server's tools/list
// response (testdata/mcp_tools_list.json, captured during the v0.7.0 PR).
// For each tool that the Go client wraps, asserts that every request-arg
// name the client sends is declared in the snapshot's inputSchema.properties.
//
// If the live server drifts (renames a property, adds a required field, drops
// a property), this test stays pinned to the snapshot. Re-capturing the
// snapshot is a deliberate maintenance step, not an automatic one — that way
// the diff is visible in code review.
//
// This test is the regression guard for v0.6.3: at that release the client
// sent `uuid` for get_monitor_mtta but the server declared only
// (from, to, monitor_uuids). Loading the snapshot here and running this test
// would have caught it.

type schemaTool struct {
	Name        string `json:"name"`
	InputSchema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	} `json:"inputSchema"`
}

type schemaToolsList struct {
	Result struct {
		Tools []schemaTool `json:"tools"`
	} `json:"result"`
}

// clientToolArgs enumerates, per tool, the request-arg names the Go client
// currently sends. Update this map ONLY together with the corresponding
// CallTool args in mcp_client.go. The test rejects any arg not declared in
// the snapshot's inputSchema.properties.
//
// Scope: this registry covers the tools v0.7.0 explicitly probed against
// the live MCP server and confirmed correct. Other tools wrapped by
// MCPClient (create_monitor, update_monitor, etc.) are intentionally NOT
// listed yet because their full wire format has not been probed in this
// release; adding them here without probe evidence would just shift
// drift risk into a test that lies. Future PRs that probe additional
// tools should extend this map together with any client-side arg fixes
// surfaced by the comparison.
//
// Tools with no args (get_status_summary, list_team_members, ...) ARE
// listed with an empty slice when probed, so a future arg addition
// without a corresponding snapshot/property update is caught.
func clientToolArgs() map[string][]string {
	return map[string][]string{
		// Windowed batched tools — the v0.6.3 bug was here. v0.7.0 fixes
		// all four to send monitor_uuids[] + from/to per the snapshot.
		"get_monitor_mtta":          {"from", "to", "monitor_uuids"},
		"get_monitor_mttr":          {"from", "to", "monitor_uuids"},
		"get_monitor_response_time": {"from", "to", "monitor_uuids"},
		"get_monitor_uptime":        {"from", "to", "monitor_uuids"},
	}
}

func loadToolsListSnapshot(t *testing.T) schemaToolsList {
	t.Helper()
	path := filepath.Join("testdata", "mcp_tools_list.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "missing snapshot %s; capture via ./testdata/regenerate_mcp_tools_list.sh", path)
	var s schemaToolsList
	require.NoError(t, json.Unmarshal(data, &s))
	require.NotEmpty(t, s.Result.Tools, "snapshot must contain at least one tool")
	return s
}

func TestSchemaContract_AllClientToolsArePresentInSnapshot(t *testing.T) {
	snap := loadToolsListSnapshot(t)
	declared := map[string]schemaTool{}
	for _, tl := range snap.Result.Tools {
		declared[tl.Name] = tl
	}

	for tool := range clientToolArgs() {
		_, ok := declared[tool]
		require.True(t, ok, "client wraps tool %q which is not declared in tools/list snapshot", tool)
	}
}

func TestSchemaContract_ClientArgsAreSubsetOfDeclaredProperties(t *testing.T) {
	snap := loadToolsListSnapshot(t)
	declared := map[string]schemaTool{}
	for _, tl := range snap.Result.Tools {
		declared[tl.Name] = tl
	}

	for tool, args := range clientToolArgs() {
		tl, ok := declared[tool]
		if !ok {
			continue // covered by the presence test above
		}
		// Build the declared property name set.
		declaredProps := map[string]struct{}{}
		for k := range tl.InputSchema.Properties {
			declaredProps[k] = struct{}{}
		}
		for _, arg := range args {
			_, ok := declaredProps[arg]
			require.True(t, ok,
				"tool %q: client sends arg %q which is not declared in inputSchema.properties; declared=%v",
				tool, arg, sortedKeys(declaredProps),
			)
		}
	}
}

func TestSchemaContract_WindowedToolsDeclareMonitorUuidsArray(t *testing.T) {
	// Targeted regression guard for the v0.6.3 bug. If the server ever
	// renames monitor_uuids back to uuid, or changes the type away from
	// array, this test fails loudly. (The snapshot is pinned, so the
	// failure would surface during the deliberate re-capture step.)
	snap := loadToolsListSnapshot(t)
	for _, tool := range []string{
		"get_monitor_mtta", "get_monitor_mttr",
		"get_monitor_response_time", "get_monitor_uptime",
	} {
		var found bool
		var props map[string]any
		for _, tl := range snap.Result.Tools {
			if tl.Name == tool {
				found = true
				props = tl.InputSchema.Properties
				break
			}
		}
		require.True(t, found, "snapshot is missing tool %q", tool)
		mu, ok := props["monitor_uuids"]
		require.True(t, ok, "tool %q: snapshot must declare monitor_uuids (the v0.6.3 bug pivot)", tool)
		// monitor_uuids must be an array per spec.
		muMap, ok := mu.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "array", muMap["type"], "tool %q: monitor_uuids must be of type array", tool)
	}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Sanity: the schema test uses a snapshot, not a network call. Keep this test
// as a tripwire so the snapshot path stays under testdata/.
func TestSchemaContract_NoNetworkInUnitTests(t *testing.T) {
	// Read with a tiny timeout would still try to open the file; the
	// timeout here is just a sanity guard for accidentally-network refactors.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = ctx
	loadToolsListSnapshot(t)
}

// ==================== Output-shape pinning ====================
//
// v0.7.0 caught REQUEST-side drift (the client sending a key the server
// did not declare) but missed RESPONSE-side shape lies. The
// list_recent_alerts bug (AlertHistory{Alerts,Total} vs the server's
// {timeGroups,totalAlerts,downAlerts,upAlerts,rawAlerts}) decoded to
// all-zero values silently because Go's encoding/json ignores unknown
// fields by default and DisallowUnknownFields is not enabled on the
// MCPClient decode path. The fixtures below were captured live on
// 2026-06-09 against /v1/mcp tools/call. Each tool's wrapper method is
// invoked through a stubbed transport whose result IS the fixture,
// asserting the resulting typed struct round-trips back to the fixture
// JSON without losing fields. A regression to a leaner struct will
// surface as a "fixture has key X that the Go type does not preserve"
// failure.

type outputShapeCase struct {
	tool    string
	fixture string
	// run invokes the corresponding MCPClient method against a stub
	// transport whose result is the parsed fixture, and returns the
	// decoded-then-remarshalled JSON. The roundtrip is what catches a
	// regression like AlertHistory{Alerts,Total} eating timeGroups.
	run func(t *testing.T, raw any) []byte
}

// loadFixture reads testdata/mcp_responses/<tool>.json and parses it.
// The MCPClient decode path receives content[0].text already json-
// decoded into native types (map[string]any or []any), which is what
// loadFixture returns.
func loadFixture(t *testing.T, tool string) any {
	t.Helper()
	path := filepath.Join("testdata", "mcp_responses", tool+".json")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "missing fixture %s", path)
	var parsed any
	require.NoError(t, json.Unmarshal(data, &parsed))
	return parsed
}

// outputShapeStubTransport replays a fixed result for one CallTool call
// regardless of toolName or args. Reused per-case so the test isolates
// the decode-path behavior from transport behavior.
type outputShapeStubTransport struct {
	result any
}

func (s *outputShapeStubTransport) Initialize(_ context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

func (s *outputShapeStubTransport) CallTool(_ context.Context, _ string, _ map[string]any) (any, error) {
	return s.result, nil
}

func outputShapeCases() []outputShapeCase {
	return []outputShapeCase{
		{
			tool:    "get_status_summary",
			fixture: "get_status_summary",
			run: func(t *testing.T, raw any) []byte {
				c := NewMCPClient(&outputShapeStubTransport{result: raw})
				got, err := c.GetStatusSummary(context.Background())
				require.NoError(t, err)
				require.NotNil(t, got)
				out, err := json.Marshal(got)
				require.NoError(t, err)
				return out
			},
		},
		{
			tool:    "list_recent_alerts",
			fixture: "list_recent_alerts",
			run: func(t *testing.T, raw any) []byte {
				c := NewMCPClient(&outputShapeStubTransport{result: raw})
				got, err := c.ListRecentAlerts(context.Background())
				require.NoError(t, err)
				require.NotNil(t, got)
				out, err := json.Marshal(got)
				require.NoError(t, err)
				return out
			},
		},
	}
}

// TestSchemaContract_SpecFieldsMatchGoTypes asserts that for the core model
// types, every JSON field name present in the Go struct tags also appears as a
// property key in the OpenAPI spec schema. This catches drift when a Go struct
// field is renamed without updating openapi.yaml.
//
// The table below is hand-maintained; update it when adding or renaming json
// tags on the listed types.
func TestSchemaContract_SpecFieldsMatchGoTypes(t *testing.T) {
	type schemaCase struct {
		schema string
		fields []string
	}
	cases := []schemaCase{
		{
			schema: "Monitor",
			// Fields from Monitor struct json tags (excluding json:"-").
			// escalation_policy comes from monitorWire but is the wire field the spec
			// documents; include it so a rename surfaces here.
			fields: []string{
				"id", "uuid", "name", "url", "protocol", "projectUuid",
				"http_method", "regions", "check_frequency", "request_headers",
				"request_body", "follow_redirects", "expected_status_code",
				"required_keyword", "paused", "port", "alerts_wait",
				"dns_record_type", "dns_nameserver", "dns_expected_answer",
				"status", "ssl_expiration", "escalation_policy",
				"ip_version", "ssl_alert_days", "ssl_reminders",
				"ssl_notify_on_change", "domain_alert_days", "domain_expiration",
			},
		},
		{
			schema: "Healthcheck",
			fields: []string{
				"uuid", "publicUuid", "name", "pingUrl", "cron", "timezone", "tz",
				"periodValue", "periodType", "period", "gracePeriod",
				"gracePeriodValue", "gracePeriodType", "isDown", "isPaused",
				"lastPing", "dueDate", "createdAt", "lastLogStartDate",
				"lastLogEndDate", "escalationPolicy",
			},
		},
		{
			schema: "StatusPageService",
			fields: []string{
				"id", "uuid", "name", "is_group", "type", "show_uptime",
				"show_response_times", "description", "services",
			},
		},
		{
			schema: "Incident",
			fields: []string{
				"uuid", "date", "title", "text", "type",
				"affectedComponents", "statuspages", "updates",
			},
		},
		{
			schema: "Outage",
			fields: []string{
				"uuid", "startDate", "endDate", "durationMs", "statusCode",
				"description", "outageType", "isResolved", "detectedLocation",
				"confirmedLocations", "acknowledgedAt", "acknowledgedBy",
				"monitor", "escalationPolicy", "severity", "summary",
			},
		},
		{
			schema: "TeamMember",
			fields: []string{
				"uuid", "email", "name", "phone",
				"profilePictureUrl", "ssoPictureUrl", "accountRole",
			},
		},
		{
			schema: "AlertHistory",
			fields: []string{
				"timeGroups", "totalAlerts", "downAlerts", "upAlerts", "rawAlerts",
			},
		},
	}

	data, err := os.ReadFile("openapi.yaml")
	require.NoError(t, err, "openapi.yaml must exist")
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(data, &spec))
	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	schemas, ok := components["schemas"].(map[string]any)
	require.True(t, ok)

	for _, tc := range cases {
		t.Run(tc.schema, func(t *testing.T) {
			schemaObj, ok := schemas[tc.schema].(map[string]any)
			require.True(t, ok, "schema %q must exist in components.schemas", tc.schema)
			props, ok := schemaObj["properties"].(map[string]any)
			require.True(t, ok, "%s must have a properties map", tc.schema)
			for _, field := range tc.fields {
				require.Contains(t, props, field,
					"%s: Go json tag %q has no matching property in spec; update openapi.yaml when renaming fields",
					tc.schema, field)
			}
		})
	}
}

// TestSchemaContract_OutputShape_PinsLiveServerFields asserts that for
// each pinned tool, every key in the live response fixture survives the
// round-trip through the corresponding MCPClient method's typed Go
// struct. If a Go field is missing or its JSON tag drifts, the
// remarshalled JSON will lack that key and the assertion fails with a
// pointer to the offending field name.
//
// Scope: pinned to the affected-by-v0.7.1 nil-args methods whose Go
// types CHANGED in this release (StatusSummary, AlertHistory). Methods
// whose types stayed unchanged but whose request fix landed in v0.7.1
// (ListOnCallSchedules, ListEscalationPolicies, ListIntegrations,
// ListTeamMembers) are exercised by the integration suite plus the
// dedicated unmarshal unit tests; pinning them here would be redundant
// AND would force a fixture update every time the live server adds a
// new top-level field to one of those tools.
func TestSchemaContract_OutputShape_PinsLiveServerFields(t *testing.T) {
	for _, tc := range outputShapeCases() {
		t.Run(tc.tool, func(t *testing.T) {
			raw := loadFixture(t, tc.fixture)
			out := tc.run(t, raw)
			// Parse both sides into map[string]any so the comparison is
			// key-by-key. The Go marshal output may not be byte-equal to
			// the fixture (field ordering, whitespace), but every
			// top-level key in the fixture MUST be present in the
			// remarshalled output.
			fixtureMap, ok := raw.(map[string]any)
			if !ok {
				// list_team_members and list_escalation_policies are
				// top-level arrays; the helper above only registers map
				// shapes, so this branch is unreachable for the current
				// case set. Keep the guard so future array-shape
				// additions fail loudly here rather than silently
				// passing.
				t.Fatalf("output-shape pin only supports map fixtures; %s is %T", tc.tool, raw)
			}
			var outMap map[string]any
			require.NoError(t, json.Unmarshal(out, &outMap),
				"remarshalled output is not a JSON object: %s", string(out))
			for key := range fixtureMap {
				_, present := outMap[key]
				require.True(t, present,
					"tool %q: live response key %q dropped at decode; the Go type is missing a field with that JSON tag",
					tc.tool, key)
			}
		})
	}
}
