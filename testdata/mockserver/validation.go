// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	hyperping "github.com/hyperping/hyperping-go"
)

// TODO(GO-07): replace structural validation with specValidator once openapi.yaml is present.

// decodeBody decodes the JSON request body into dst.
func decodeBody(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	return json.NewDecoder(r.Body).Decode(dst)
}

// validateMonitorCreate checks required fields for monitor creation.
func validateMonitorCreate(req hyperping.CreateMonitorRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.URL == "" {
		return fmt.Errorf("url is required")
	}
	if req.Protocol == "" {
		return fmt.Errorf("protocol is required")
	}
	return nil
}

// validateHealthcheckCreate checks required fields for healthcheck creation.
func validateHealthcheckCreate(req hyperping.CreateHealthcheckRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// validateIncidentCreate checks required fields for incident creation.
func validateIncidentCreate(req hyperping.CreateIncidentRequest) error {
	if isEmptyLocalizedText(req.Title) {
		return fmt.Errorf("title is required")
	}
	return nil
}

// validateMaintenanceCreate checks required fields for maintenance window creation.
func validateMaintenanceCreate(req hyperping.CreateMaintenanceRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.StartDate == "" {
		return fmt.Errorf("start_date is required")
	}
	if req.EndDate == "" {
		return fmt.Errorf("end_date is required")
	}
	return nil
}

// validateOutageCreate checks required fields for outage creation.
func validateOutageCreate(req hyperping.CreateOutageRequest) error {
	if req.MonitorUUID == "" {
		return fmt.Errorf("monitorUuid is required")
	}
	if req.StartDate == "" {
		return fmt.Errorf("startDate is required")
	}
	return nil
}

// validateStatusPageCreate checks required fields for status page creation.
func validateStatusPageCreate(req hyperping.CreateStatusPageRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// validateSubscriberAdd checks required fields for subscriber addition.
func validateSubscriberAdd(req hyperping.AddSubscriberRequest) error {
	switch req.Type {
	case "email":
		if req.Email == nil || *req.Email == "" {
			return fmt.Errorf("email is required when type is 'email'")
		}
	case "sms":
		if req.Phone == nil || *req.Phone == "" {
			return fmt.Errorf("phone is required when type is 'sms'")
		}
	case "teams":
		if req.TeamsWebhookURL == nil || *req.TeamsWebhookURL == "" {
			return fmt.Errorf("teams_webhook_url is required when type is 'teams'")
		}
	default:
		return fmt.Errorf("type is required (email, sms, or teams)")
	}
	return nil
}

// isEmptyLocalizedText returns true if all locale fields are empty.
func isEmptyLocalizedText(t hyperping.LocalizedText) bool {
	return t.En == "" && t.Fr == "" && t.De == "" && t.Ru == "" &&
		t.Nl == "" && t.Es == "" && t.It == "" && t.Pt == "" &&
		t.Ja == "" && t.Zh == ""
}
