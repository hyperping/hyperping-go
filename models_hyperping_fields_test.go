// Copyright (c) 2026 Hyperping
// SPDX-License-Identifier: MIT

package hyperping

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Healthcheck: timezone ("tz" vs "timezone") and publicUuid
// =============================================================================

func TestHealthcheck_UnmarshalJSON_Timezone(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			// GET /v2/healthchecks and GET/PUT /v2/healthchecks/{uuid} return "tz".
			name: "GET shape: tz only",
			body: `{"uuid":"tok_a","name":"cron","cron":"0 3 * * *","tz":"Europe/Berlin"}`,
			want: "Europe/Berlin",
		},
		{
			// POST /v2/healthchecks returns "timezone".
			name: "POST shape: timezone only",
			body: `{"uuid":"tok_a","name":"cron","cron":"0 3 * * *","timezone":"America/New_York"}`,
			want: "America/New_York",
		},
		{
			name: "both present: tz (the stored column) wins",
			body: `{"uuid":"tok_a","name":"cron","tz":"Europe/Paris","timezone":"UTC"}`,
			want: "Europe/Paris",
		},
		{
			name: "period-based: tz null",
			body: `{"uuid":"tok_a","name":"period","tz":null,"periodValue":5,"periodType":"minutes"}`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hc Healthcheck
			require.NoError(t, json.Unmarshal([]byte(tt.body), &hc))
			assert.Equal(t, tt.want, hc.Timezone, "Timezone")
			assert.Equal(t, tt.want, hc.Tz, "Tz")
			assert.Equal(t, tt.want, hc.GetTimezone(), "GetTimezone()")
		})
	}
}

func TestHealthcheck_GetTimezone_PrefersTz(t *testing.T) {
	// Values built in Go (not decoded) keep both fields as set.
	assert.Equal(t, "Europe/Berlin", Healthcheck{Tz: "Europe/Berlin", Timezone: "UTC"}.GetTimezone())
	assert.Equal(t, "UTC", Healthcheck{Timezone: "UTC"}.GetTimezone())
	assert.Equal(t, "", Healthcheck{}.GetTimezone())
}

func TestHealthcheck_UnmarshalJSON_PublicUUID(t *testing.T) {
	var with Healthcheck
	require.NoError(t, json.Unmarshal([]byte(`{"uuid":"tok_a","publicUuid":"hc_b","name":"x"}`), &with))
	require.NotNil(t, with.PublicUUID)
	assert.Equal(t, "hc_b", *with.PublicUUID)
	assert.Equal(t, "tok_a", with.UUID)

	var without Healthcheck
	require.NoError(t, json.Unmarshal([]byte(`{"uuid":"tok_a","name":"x"}`), &without))
	assert.Nil(t, without.PublicUUID, "absent publicUuid (older API) must stay nil")
}

func TestHealthcheck_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var hc Healthcheck
	assert.Error(t, json.Unmarshal([]byte(`{"uuid":`), &hc))
}

// TestClient_GetHealthcheck_CronTimezoneFromTz reproduces the provider bug
// "timezone was Europe/Berlin, but now null": GET returns "tz", and a consumer
// reading Healthcheck.Timezone must still see it.
func TestClient_GetHealthcheck_CronTimezoneFromTz(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthcheck":{"uuid":"tok_cron","publicUuid":"hc_cron","name":"nightly","cron":"0 3 * * *","tz":"Europe/Berlin","gracePeriod":600,"gracePeriodValue":10,"gracePeriodType":"minutes"}}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithHTTPClient(server.Client()), WithBaseURL(server.URL), WithMaxRetries(0))
	hc, err := client.GetHealthcheck(context.Background(), "tok_cron")
	require.NoError(t, err)
	assert.Equal(t, "Europe/Berlin", hc.Timezone)
	assert.Equal(t, "Europe/Berlin", hc.GetTimezone())
	require.NotNil(t, hc.PublicUUID)
	assert.Equal(t, "hc_cron", *hc.PublicUUID)
}

func TestClient_ListHealthchecks_TimezoneFromTz(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthchecks":[{"uuid":"tok_1","publicUuid":"hc_1","name":"a","cron":"0 3 * * *","tz":"Europe/Berlin"},{"uuid":"tok_2","name":"b","tz":null,"periodValue":1,"periodType":"hours"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithHTTPClient(server.Client()), WithBaseURL(server.URL), WithMaxRetries(0))
	list, err := client.ListHealthchecks(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "Europe/Berlin", list[0].Timezone)
	require.NotNil(t, list[0].PublicUUID)
	assert.Equal(t, "hc_1", *list[0].PublicUUID)
	assert.Equal(t, "", list[1].Timezone)
	assert.Nil(t, list[1].PublicUUID)
}

// =============================================================================
// Monitor: ip_version and TLS certificate / domain expiry alerting
// =============================================================================

func TestMonitor_UnmarshalJSON_ExpiryAndIPVersion(t *testing.T) {
	body := `{"uuid":"mon_a","name":"x","url":"https://example.com","protocol":"http",
		"ip_version":6,"ssl_expiration":41,"ssl_alert_days":30,"ssl_reminders":true,
		"ssl_notify_on_change":false,"domain_alert_days":-1,"domain_expiration":null,
		"escalation_policy":{"uuid":"ep_1","name":"Default"}}`
	var m Monitor
	require.NoError(t, json.Unmarshal([]byte(body), &m))
	require.NotNil(t, m.IPVersion)
	assert.Equal(t, 6, *m.IPVersion)
	require.NotNil(t, m.SSLExpiration)
	assert.Equal(t, 41, *m.SSLExpiration)
	require.NotNil(t, m.SSLAlertDays)
	assert.Equal(t, 30, *m.SSLAlertDays)
	require.NotNil(t, m.SSLReminders)
	assert.True(t, *m.SSLReminders)
	require.NotNil(t, m.SSLNotifyOnChange)
	assert.False(t, *m.SSLNotifyOnChange)
	require.NotNil(t, m.DomainAlertDays)
	assert.Equal(t, -1, *m.DomainAlertDays)
	assert.Nil(t, m.DomainExpiration)
	require.NotNil(t, m.EscalationPolicy, "custom UnmarshalJSON must still decode escalation_policy")
	assert.Equal(t, "ep_1", m.EscalationPolicy.UUID)
}

func TestMonitor_UnmarshalJSON_ExpiryFieldsAbsent(t *testing.T) {
	var m Monitor
	require.NoError(t, json.Unmarshal([]byte(`{"uuid":"mon_a","name":"x"}`), &m))
	assert.Nil(t, m.IPVersion)
	assert.Nil(t, m.SSLAlertDays)
	assert.Nil(t, m.SSLReminders)
	assert.Nil(t, m.SSLNotifyOnChange)
	assert.Nil(t, m.DomainAlertDays)
	assert.Nil(t, m.DomainExpiration)
}

func TestMonitorRequests_ExpiryAndIPVersion_JSON(t *testing.T) {
	t.Run("omitted when nil", func(t *testing.T) {
		for _, v := range []any{
			CreateMonitorRequest{Name: "x", URL: "https://example.com", Protocol: "http"},
			UpdateMonitorRequest{Name: strPtr("x")},
		} {
			out, err := json.Marshal(v)
			require.NoError(t, err)
			for _, key := range []string{"ip_version", "ssl_alert_days", "ssl_reminders", "ssl_notify_on_change", "domain_alert_days"} {
				assert.NotContains(t, string(out), `"`+key+`"`)
			}
		}
	})
	t.Run("sent when set, including false and -1", func(t *testing.T) {
		create := CreateMonitorRequest{
			Name: "x", URL: "https://example.com", Protocol: "http",
			IPVersion: intPtr(6), SSLAlertDays: intPtr(-1), SSLReminders: boolPtr(false),
			SSLNotifyOnChange: boolPtr(true), DomainAlertDays: intPtr(30),
		}
		update := UpdateMonitorRequest{
			IPVersion: intPtr(4), SSLAlertDays: intPtr(7), SSLReminders: boolPtr(true),
			SSLNotifyOnChange: boolPtr(false), DomainAlertDays: intPtr(-1),
		}
		var got map[string]any
		out, err := json.Marshal(create)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(out, &got))
		assert.EqualValues(t, 6, got["ip_version"])
		assert.EqualValues(t, -1, got["ssl_alert_days"])
		assert.Equal(t, false, got["ssl_reminders"])
		assert.Equal(t, true, got["ssl_notify_on_change"])
		assert.EqualValues(t, 30, got["domain_alert_days"])

		got = nil
		out, err = json.Marshal(update)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(out, &got))
		assert.EqualValues(t, 4, got["ip_version"])
		assert.EqualValues(t, 7, got["ssl_alert_days"])
		assert.Equal(t, true, got["ssl_reminders"])
		assert.Equal(t, false, got["ssl_notify_on_change"])
		assert.EqualValues(t, -1, got["domain_alert_days"])
	})
}

func TestAllowedExpiryAndIPVersionValues(t *testing.T) {
	assert.Equal(t, []int{4, 6}, AllowedIPVersions)
	assert.Equal(t, []int{-1, 1, 3, 7, 15, 30, 60, 90}, AllowedSSLAlertDays)
	assert.Equal(t, []int{-1, 7, 14, 30, 60, 90}, AllowedDomainAlertDays)
}

// =============================================================================
// Status page services: read-only type, healthchecks (hc_) in sections/groups
// =============================================================================

func TestStatusPageService_Type_JSON(t *testing.T) {
	body := `{"name":{"en":"Main"},"is_split":false,"services":[
		{"id":"mon_a","uuid":"mon_a","name":{"en":"API"},"is_group":false,"type":"monitor","show_uptime":true,"show_response_times":true},
		{"id":"hc_b","uuid":"hc_b","name":{"en":"Nightly"},"is_group":false,"type":"healthcheck","show_uptime":true,"show_response_times":false},
		{"uuid":"grp_1","name":{"en":"Group"},"is_group":true,"show_uptime":true,"show_response_times":true,"services":[
			{"id":"hc_c","uuid":"hc_c","name":{"en":"Backup"},"is_group":false,"type":"healthcheck","show_uptime":false,"show_response_times":false},
			{"id":"agt_d","uuid":"agt_d","name":{"en":"Box"},"is_group":false,"type":"server","show_uptime":true,"show_response_times":false}
		]}
	]}`
	var section StatusPageSection
	require.NoError(t, json.Unmarshal([]byte(body), &section))
	require.Len(t, section.Services, 3)
	assert.Equal(t, "monitor", section.Services[0].Type)
	assert.Equal(t, "healthcheck", section.Services[1].Type)
	assert.Equal(t, "", section.Services[2].Type, "group header has no type")
	require.Len(t, section.Services[2].Services, 2)
	assert.Equal(t, "healthcheck", section.Services[2].Services[0].Type)
	assert.Equal(t, "server", section.Services[2].Services[1].Type)

	// type is read-only: an empty value is not marshalled back.
	out, err := json.Marshal(StatusPageService{UUID: "mon_a", Name: map[string]string{"en": "API"}})
	require.NoError(t, err)
	assert.NotContains(t, string(out), `"type"`)
}

func TestCreateStatusPageService_HealthcheckChildFlags_JSON(t *testing.T) {
	// A healthcheck inside a group is a nested child: it is referenced by
	// "uuid" and its show_uptime/show_response_times flags must be sent even
	// when false.
	group := CreateStatusPageService{
		NameShown: strPtr("Jobs"),
		IsGroup:   boolPtr(true),
		Services: []CreateStatusPageService{{
			UUID:              strPtr("hc_c"),
			Name:              map[string]string{"en": "Backup"},
			ShowUptime:        boolPtr(false),
			ShowResponseTimes: boolPtr(false),
		}},
	}
	out, err := json.Marshal(group)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	children, ok := got["services"].([]any)
	require.True(t, ok)
	require.Len(t, children, 1)
	child, ok := children[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "hc_c", child["uuid"])
	assert.Equal(t, false, child["show_uptime"])
	assert.Equal(t, false, child["show_response_times"])
}
