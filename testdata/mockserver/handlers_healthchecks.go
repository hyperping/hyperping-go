// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"net/http"

	hyperping "github.com/hyperping/hyperping-go"
)

func registerHealthcheckHandlers(mux *http.ServeMux, store *mockStore) {
	mux.HandleFunc("GET /v2/healthchecks", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		list := make([]hyperping.Healthcheck, 0, len(store.healthchecks))
		for _, hc := range store.healthchecks {
			list = append(list, *hc)
		}
		store.mu.RUnlock()
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /v2/healthchecks", func(w http.ResponseWriter, r *http.Request) {
		var req hyperping.CreateHealthcheckRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := validateHealthcheckCreate(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		hc := hyperping.Healthcheck{
			UUID: newUUID(),
			Name: req.Name,
		}
		if req.PeriodValue != nil {
			hc.PeriodValue = req.PeriodValue
		}
		if req.PeriodType != nil {
			hc.PeriodType = *req.PeriodType
		}
		if req.GracePeriodValue != 0 {
			hc.GracePeriodValue = req.GracePeriodValue
		}
		if req.GracePeriodType != "" {
			hc.GracePeriodType = req.GracePeriodType
		}
		store.mu.Lock()
		store.healthchecks[hc.UUID] = &hc
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, hc)
	})

	mux.HandleFunc("GET /v2/healthchecks/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		hc, ok := store.healthchecks[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "healthcheck not found")
			return
		}
		writeJSON(w, http.StatusOK, *hc)
	})

	mux.HandleFunc("PUT /v2/healthchecks/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		hc, ok := store.healthchecks[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "healthcheck not found")
			return
		}
		var req hyperping.UpdateHealthcheckRequest
		if err := decodeBody(r, &req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if req.Name != nil {
			hc.Name = *req.Name
		}
		updated := *hc
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("DELETE /v2/healthchecks/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		_, ok := store.healthchecks[uuid]
		if ok {
			delete(store.healthchecks, uuid)
		}
		store.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "healthcheck not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /v2/healthchecks/{uuid}/pause", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		_, ok := store.healthchecks[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "healthcheck not found")
			return
		}
		writeJSON(w, http.StatusOK, hyperping.HealthcheckAction{
			Message: "Healthcheck paused",
			UUID:    uuid,
		})
	})

	mux.HandleFunc("POST /v2/healthchecks/{uuid}/resume", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		_, ok := store.healthchecks[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "healthcheck not found")
			return
		}
		writeJSON(w, http.StatusOK, hyperping.HealthcheckAction{
			Message: "Healthcheck resumed",
			UUID:    uuid,
		})
	})
}
