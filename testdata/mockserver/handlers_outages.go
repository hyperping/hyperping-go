// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"net/http"

	hyperping "github.com/hyperping/hyperping-go"
)

func registerOutageHandlers(mux *http.ServeMux, store *mockStore) {
	mux.HandleFunc("GET /v2/outages", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		list := make([]hyperping.Outage, 0, len(store.outages))
		for _, o := range store.outages {
			list = append(list, *o)
		}
		store.mu.RUnlock()
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /v2/outages", func(w http.ResponseWriter, r *http.Request) {
		var req hyperping.CreateOutageRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := validateOutageCreate(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		o := hyperping.Outage{
			UUID:      newUUID(),
			StartDate: req.StartDate,
			EndDate:   req.EndDate,
			Monitor: hyperping.MonitorReference{
				UUID: req.MonitorUUID,
			},
			OutageType:  req.OutageType,
			StatusCode:  req.StatusCode,
			Description: req.Description,
		}
		store.mu.Lock()
		store.outages[o.UUID] = &o
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, o)
	})

	mux.HandleFunc("GET /v2/outages/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		o, ok := store.outages[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "outage not found")
			return
		}
		writeJSON(w, http.StatusOK, *o)
	})

	mux.HandleFunc("DELETE /v2/outages/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		_, ok := store.outages[uuid]
		if ok {
			delete(store.outages, uuid)
		}
		store.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "outage not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /v2/outages/{uuid}/acknowledge", makeOutageActionHandler(store, "Outage acknowledged"))
	mux.HandleFunc("POST /v2/outages/{uuid}/unacknowledge", makeOutageActionHandler(store, "Outage unacknowledged"))
	mux.HandleFunc("POST /v2/outages/{uuid}/resolve", makeOutageActionHandler(store, "Outage resolved"))
	mux.HandleFunc("POST /v2/outages/{uuid}/escalate", makeOutageActionHandler(store, "Outage escalated"))
}

func makeOutageActionHandler(store *mockStore, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		_, ok := store.outages[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "outage not found")
			return
		}
		writeJSON(w, http.StatusOK, hyperping.OutageAction{
			Message: message,
			UUID:    uuid,
		})
	}
}
