// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"net/http"

	hyperping "github.com/hyperping/hyperping-go"
)

func registerMaintenanceHandlers(mux *http.ServeMux, store *mockStore) {
	mux.HandleFunc("GET /v1/maintenance-windows", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		list := make([]hyperping.Maintenance, 0, len(store.maintenance))
		for _, m := range store.maintenance {
			list = append(list, *m)
		}
		store.mu.RUnlock()
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /v1/maintenance-windows", func(w http.ResponseWriter, r *http.Request) {
		var req hyperping.CreateMaintenanceRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := validateMaintenanceCreate(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		mw := hyperping.Maintenance{
			UUID:      newUUID(),
			Name:      req.Name,
			Title:     req.Title,
			Text:      req.Text,
			StartDate: &req.StartDate,
			EndDate:   &req.EndDate,
			Monitors:  req.Monitors,
		}
		store.mu.Lock()
		store.maintenance[mw.UUID] = &mw
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, mw)
	})

	mux.HandleFunc("GET /v1/maintenance-windows/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		mw, ok := store.maintenance[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "maintenance window not found")
			return
		}
		writeJSON(w, http.StatusOK, *mw)
	})

	mux.HandleFunc("PUT /v1/maintenance-windows/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		mw, ok := store.maintenance[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "maintenance window not found")
			return
		}
		var req hyperping.UpdateMaintenanceRequest
		if err := decodeBody(r, &req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if req.Name != nil {
			mw.Name = *req.Name
		}
		if req.StartDate != nil {
			mw.StartDate = req.StartDate
		}
		if req.EndDate != nil {
			mw.EndDate = req.EndDate
		}
		updated := *mw
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("DELETE /v1/maintenance-windows/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		_, ok := store.maintenance[uuid]
		if ok {
			delete(store.maintenance, uuid)
		}
		store.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "maintenance window not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
