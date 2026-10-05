// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"net/http"
	"sort"

	hyperping "github.com/hyperping/hyperping-go"
)

func registerMonitorHandlers(mux *http.ServeMux, store *mockStore) {
	mux.HandleFunc("GET /v1/monitors", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		list := make([]hyperping.Monitor, 0, len(store.monitors))
		for _, m := range store.monitors {
			list = append(list, *m)
		}
		store.mu.RUnlock()
		sort.Slice(list, func(i, j int) bool { return list[i].UUID < list[j].UUID })
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /v1/monitors", func(w http.ResponseWriter, r *http.Request) {
		var req hyperping.CreateMonitorRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := validateMonitorCreate(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		m := hyperping.Monitor{
			UUID:     newUUID(),
			Name:     req.Name,
			URL:      req.URL,
			Protocol: req.Protocol,
		}
		store.mu.Lock()
		store.monitors[m.UUID] = &m
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, m)
	})

	mux.HandleFunc("GET /v1/monitors/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		m, ok := store.monitors[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		writeJSON(w, http.StatusOK, *m)
	})

	mux.HandleFunc("PUT /v1/monitors/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		m, ok := store.monitors[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		var req hyperping.UpdateMonitorRequest
		if err := decodeBody(r, &req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		applyMonitorUpdate(m, req)
		updated := *m
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("DELETE /v1/monitors/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		_, ok := store.monitors[uuid]
		if ok {
			delete(store.monitors, uuid)
		}
		store.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /v1/monitors/{uuid}/pause", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		m, ok := store.monitors[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		m.Paused = true
		updated := *m
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("POST /v1/monitors/{uuid}/resume", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		m, ok := store.monitors[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		m.Paused = false
		updated := *m
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	})
}

func applyMonitorUpdate(m *hyperping.Monitor, req hyperping.UpdateMonitorRequest) {
	if req.Name != nil {
		m.Name = *req.Name
	}
	if req.URL != nil {
		m.URL = *req.URL
	}
	if req.Protocol != nil {
		m.Protocol = *req.Protocol
	}
	if req.Paused != nil {
		m.Paused = *req.Paused
	}
	if req.CheckFrequency != nil {
		m.CheckFrequency = *req.CheckFrequency
	}
}
