// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"net/http"
	"time"

	hyperping "github.com/hyperping/hyperping-go"
)

func registerIncidentHandlers(mux *http.ServeMux, store *mockStore) {
	mux.HandleFunc("GET /v3/incidents", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		list := make([]hyperping.Incident, 0, len(store.incidents))
		for _, inc := range store.incidents {
			list = append(list, *inc)
		}
		store.mu.RUnlock()
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /v3/incidents", func(w http.ResponseWriter, r *http.Request) {
		var req hyperping.CreateIncidentRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := validateIncidentCreate(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		date := req.Date
		if date == "" {
			date = time.Now().UTC().Format(time.RFC3339)
		}
		inc := hyperping.Incident{
			UUID:               newUUID(),
			Title:              req.Title,
			Text:               req.Text,
			Type:               req.Type,
			AffectedComponents: req.AffectedComponents,
			StatusPages:        req.StatusPages,
			Date:               date,
		}
		store.mu.Lock()
		store.incidents[inc.UUID] = &inc
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, inc)
	})

	mux.HandleFunc("GET /v3/incidents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		inc, ok := store.incidents[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		writeJSON(w, http.StatusOK, *inc)
	})

	mux.HandleFunc("PUT /v3/incidents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		inc, ok := store.incidents[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		var req hyperping.UpdateIncidentRequest
		if err := decodeBody(r, &req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if req.Title != nil {
			inc.Title = *req.Title
		}
		if req.Text != nil {
			inc.Text = *req.Text
		}
		if req.Type != nil {
			inc.Type = *req.Type
		}
		updated := *inc
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("DELETE /v3/incidents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		_, ok := store.incidents[uuid]
		if ok {
			delete(store.incidents, uuid)
		}
		store.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /v3/incidents/{uuid}/updates", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		inc, ok := store.incidents[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		var req hyperping.AddIncidentUpdateRequest
		if err := decodeBody(r, &req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		update := hyperping.IncidentUpdate{
			UUID: newUUID(),
			Date: req.Date,
			Text: req.Text,
			Type: req.Type,
		}
		inc.Updates = append(inc.Updates, update)
		updated := *inc
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	})
}
