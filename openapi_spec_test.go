// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package hyperping

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func readOASSpec(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("openapi.yaml")
	require.NoError(t, err, "openapi.yaml must exist at repo root")
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(data, &spec), "openapi.yaml must parse as valid YAML")
	return spec
}

func TestOASSpec_Exists(t *testing.T) {
	_, err := os.Stat("openapi.yaml")
	require.NoError(t, err, "openapi.yaml must exist at repo root")
}

func TestOASSpec_ParsesYAML(t *testing.T) {
	readOASSpec(t)
}

func TestOASSpec_Version(t *testing.T) {
	spec := readOASSpec(t)
	require.Equal(t, "3.1.0", spec["openapi"], "openapi version must be 3.1.0")
}

func TestOASSpec_ServerURL(t *testing.T) {
	spec := readOASSpec(t)
	servers, ok := spec["servers"].([]any)
	require.True(t, ok && len(servers) > 0, "spec must have at least one server")
	server, ok := servers[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://api.hyperping.io", server["url"])
}

func TestOASSpec_AllClientOperationsPresent(t *testing.T) {
	expectedIDs := []string{
		// monitors
		"listMonitors", "createMonitor", "getMonitor", "updateMonitor", "deleteMonitor",
		// healthchecks
		"listHealthchecks", "createHealthcheck", "getHealthcheck", "updateHealthcheck",
		"deleteHealthcheck", "pauseHealthcheck", "resumeHealthcheck",
		// incidents
		"listIncidents", "createIncident", "getIncident", "updateIncident",
		"deleteIncident", "addIncidentUpdate",
		// maintenance
		"listMaintenanceWindows", "createMaintenanceWindow", "getMaintenanceWindow",
		"updateMaintenanceWindow", "deleteMaintenanceWindow",
		// outages
		"listOutages", "createOutage", "getOutage", "deleteOutage",
		"acknowledgeOutage", "unacknowledgeOutage", "resolveOutage", "escalateOutage",
		// reports
		"listMonitorReports", "getMonitorReport",
		// statuspages
		"listStatusPages", "createStatusPage", "getStatusPage",
		"updateStatusPage", "deleteStatusPage",
		// subscribers
		"listSubscribers", "addSubscriber", "getSubscriber", "deleteSubscriber",
		// sso connections
		"listSSOConnections", "createSSOConnection", "getSSOConnection",
		"updateSSOConnection", "deleteSSOConnection",
	}
	require.Len(t, expectedIDs, 47)

	spec := readOASSpec(t)
	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok, "spec must have paths")

	collected := map[string]struct{}{}
	httpMethods := []string{"get", "post", "put", "patch", "delete", "head", "options"}
	for _, pathItem := range paths {
		item, ok := pathItem.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			if id, ok := op["operationId"].(string); ok {
				collected[id] = struct{}{}
			}
		}
	}

	for _, id := range expectedIDs {
		require.Contains(t, collected, id, "operationId %q not found in spec", id)
	}
}

func TestOASSpec_AllMCPToolsHaveDiscriminatorBranch(t *testing.T) {
	type toolsList struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	data, err := os.ReadFile("testdata/mcp_tools_list.json")
	require.NoError(t, err)
	var tl toolsList
	require.NoError(t, json.Unmarshal(data, &tl))
	require.Len(t, tl.Result.Tools, 26)

	expectedTools := map[string]struct{}{}
	for _, tool := range tl.Result.Tools {
		expectedTools[tool.Name] = struct{}{}
	}

	spec := readOASSpec(t)
	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok)
	mcpPath, ok := paths["/v1/mcp"].(map[string]any)
	require.True(t, ok, "spec must have path /v1/mcp")
	post, ok := mcpPath["post"].(map[string]any)
	require.True(t, ok, "/v1/mcp must have POST operation")
	rb, ok := post["requestBody"].(map[string]any)
	require.True(t, ok)
	content, ok := rb["content"].(map[string]any)
	require.True(t, ok)
	appJSON, ok := content["application/json"].(map[string]any)
	require.True(t, ok)
	schema, ok := appJSON["schema"].(map[string]any)
	require.True(t, ok)
	oneOf, ok := schema["oneOf"].([]any)
	require.True(t, ok, "MCP request body schema must use oneOf")

	foundTools := map[string]struct{}{}
	for _, branch := range oneOf {
		b, ok := branch.(map[string]any)
		if !ok {
			continue
		}
		props, ok := b["properties"].(map[string]any)
		if !ok {
			continue
		}
		methodProp, ok := props["method"].(map[string]any)
		if !ok {
			continue
		}
		enumVals, ok := methodProp["enum"].([]any)
		if !ok || len(enumVals) == 0 {
			continue
		}
		if name, ok := enumVals[0].(string); ok {
			foundTools[name] = struct{}{}
		}
	}

	for toolName := range expectedTools {
		require.Contains(t, foundTools, toolName, "MCP tool %q has no discriminator branch in oneOf", toolName)
	}
}

func TestOASSpec_NoNullableKeyword(t *testing.T) {
	data, err := os.ReadFile("openapi.yaml")
	require.NoError(t, err)
	require.False(t, bytes.Contains(data, []byte("nullable: true")),
		"spec must not use 'nullable: true' (OAS 2.x style); use type: [\"string\",\"null\"] instead")
}

func TestOASSpec_FlexibleStringDef(t *testing.T) {
	spec := readOASSpec(t)
	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	schemas, ok := components["schemas"].(map[string]any)
	require.True(t, ok)
	fs, ok := schemas["FlexibleString"].(map[string]any)
	require.True(t, ok, "components.schemas.FlexibleString must exist")
	oneOf, ok := fs["oneOf"].([]any)
	require.True(t, ok && len(oneOf) == 2, "FlexibleString must have oneOf with exactly 2 branches")

	types := map[string]bool{}
	for _, branch := range oneOf {
		b, ok := branch.(map[string]any)
		require.True(t, ok)
		typ, ok := b["type"].(string)
		require.True(t, ok)
		types[typ] = true
	}
	require.True(t, types["string"], "FlexibleString oneOf must include {type: string}")
	require.True(t, types["number"], "FlexibleString oneOf must include {type: number}")
}

func TestOASSpec_EscalationPolicyPolymorphism(t *testing.T) {
	spec := readOASSpec(t)
	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	schemas, ok := components["schemas"].(map[string]any)
	require.True(t, ok)

	ref, ok := schemas["EscalationPolicyRef"].(map[string]any)
	require.True(t, ok, "components.schemas.EscalationPolicyRef must exist")
	refProps, ok := ref["properties"].(map[string]any)
	require.True(t, ok, "EscalationPolicyRef must have properties")
	require.Contains(t, refProps, "uuid", "EscalationPolicyRef must have uuid property")
	require.Contains(t, refProps, "name", "EscalationPolicyRef must have name property")

	write, ok := schemas["EscalationPolicyWrite"].(map[string]any)
	require.True(t, ok, "components.schemas.EscalationPolicyWrite must exist")
	require.Equal(t, "string", write["type"], "EscalationPolicyWrite must be type: string")
}

func TestOASSpec_NullableFieldsUseTupleType(t *testing.T) {
	spec := readOASSpec(t)
	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	schemas, ok := components["schemas"].(map[string]any)
	require.True(t, ok)

	cases := []struct {
		schema string
		field  string
	}{
		{"Outage", "endDate"},
		{"Outage", "acknowledgedAt"},
		{"TeamMember", "ssoPictureUrl"},
		{"StatusPage", "hostname"},
		{"SSOConnection", "issuer"},
		{"SSOConnection", "client_id"},
	}

	for _, tc := range cases {
		schemaObj, ok := schemas[tc.schema].(map[string]any)
		require.True(t, ok, "schema %q must exist", tc.schema)
		props, ok := schemaObj["properties"].(map[string]any)
		require.True(t, ok, "%s must have properties", tc.schema)
		fieldSchema, ok := props[tc.field].(map[string]any)
		require.True(t, ok, "%s.%s must exist in spec", tc.schema, tc.field)

		typeVal := fieldSchema["type"]
		switch v := typeVal.(type) {
		case []any:
			typeStrs := make([]string, len(v))
			for i, item := range v {
				s, ok := item.(string)
				require.True(t, ok)
				typeStrs[i] = s
			}
			hasString := false
			hasNull := false
			for _, s := range typeStrs {
				if s == "string" {
					hasString = true
				}
				if s == "null" {
					hasNull = true
				}
			}
			require.True(t, hasString && hasNull,
				"%s.%s must use tuple type [\"string\",\"null\"], got %v", tc.schema, tc.field, typeStrs)
		default:
			t.Errorf("%s.%s must use tuple type [\"string\",\"null\"], got scalar %v", tc.schema, tc.field, typeVal)
		}
	}
}
