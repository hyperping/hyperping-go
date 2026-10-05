// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"net/http"
	"strconv"
	"time"

	hyperping "github.com/hyperping/hyperping-go"
)

func registerStatusPageHandlers(mux *http.ServeMux, store *mockStore) {
	mux.HandleFunc("GET /v2/statuspages", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		list := make([]hyperping.StatusPage, 0, len(store.statusPages))
		for _, sp := range store.statusPages {
			list = append(list, *sp)
		}
		store.mu.RUnlock()
		resp := hyperping.StatusPagePaginatedResponse{
			StatusPages:    list,
			Total:          len(list),
			HasNextPage:    false,
			Page:           0,
			ResultsPerPage: 25,
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v2/statuspages", func(w http.ResponseWriter, r *http.Request) {
		var req hyperping.CreateStatusPageRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := validateStatusPageCreate(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sp := hyperping.StatusPage{
			UUID: newUUID(),
			Name: req.Name,
			Settings: hyperping.StatusPageSettings{
				Name: req.Name,
			},
		}
		if req.Hostname != nil {
			sp.Hostname = req.Hostname
		}
		store.mu.Lock()
		store.statusPages[sp.UUID] = &sp
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"message":    "Status page created",
			"statuspage": sp,
		})
	})

	mux.HandleFunc("GET /v2/statuspages/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		sp, ok := store.statusPages[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "status page not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"statuspage": *sp,
		})
	})

	mux.HandleFunc("PUT /v2/statuspages/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		sp, ok := store.statusPages[uuid]
		if !ok {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "status page not found")
			return
		}
		var req hyperping.UpdateStatusPageRequest
		if err := decodeBody(r, &req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if req.Name != nil {
			sp.Name = *req.Name
			sp.Settings.Name = *req.Name
		}
		updated := *sp
		store.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":    "Status page updated",
			"statuspage": updated,
		})
	})

	mux.HandleFunc("DELETE /v2/statuspages/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		_, ok := store.statusPages[uuid]
		if ok {
			delete(store.statusPages, uuid)
			delete(store.subscribers, uuid)
		}
		store.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "status page not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /v2/statuspages/{uuid}/subscribers", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		_, spOK := store.statusPages[uuid]
		subs := append([]hyperping.StatusPageSubscriber(nil), store.subscribers[uuid]...)
		store.mu.RUnlock()
		if !spOK {
			writeError(w, http.StatusNotFound, "status page not found")
			return
		}
		resp := hyperping.SubscriberPaginatedResponse{
			Subscribers:    subs,
			Total:          len(subs),
			HasNextPage:    false,
			Page:           0,
			ResultsPerPage: 25,
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v2/statuspages/{uuid}/subscribers", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.Lock()
		_, spOK := store.statusPages[uuid]
		if !spOK {
			store.mu.Unlock()
			writeError(w, http.StatusNotFound, "status page not found")
			return
		}
		var req hyperping.AddSubscriberRequest
		if err := decodeBody(r, &req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := validateSubscriberAdd(req); err != nil {
			store.mu.Unlock()
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Assign next ID
		nextID := len(store.subscribers[uuid]) + 1
		sub := hyperping.StatusPageSubscriber{
			ID:        nextID,
			Type:      req.Type,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if req.Email != nil {
			sub.Email = req.Email
			sub.Value = *req.Email
		}
		if req.Phone != nil {
			sub.Phone = req.Phone
			sub.Value = *req.Phone
		}
		store.subscribers[uuid] = append(store.subscribers[uuid], sub)
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"message":    "Subscriber added",
			"subscriber": sub,
		})
	})

	mux.HandleFunc("GET /v2/statuspages/{uuid}/subscribers/{id}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		idStr := r.PathValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid subscriber id")
			return
		}
		store.mu.RLock()
		subs := store.subscribers[uuid]
		store.mu.RUnlock()
		for _, s := range subs {
			if s.ID == id {
				writeJSON(w, http.StatusOK, s)
				return
			}
		}
		writeError(w, http.StatusNotFound, "subscriber not found")
	})

	mux.HandleFunc("DELETE /v2/statuspages/{uuid}/subscribers/{id}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		idStr := r.PathValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid subscriber id")
			return
		}
		store.mu.Lock()
		subs := store.subscribers[uuid]
		found := false
		updated := subs[:0]
		for _, s := range subs {
			if s.ID == id {
				found = true
			} else {
				updated = append(updated, s)
			}
		}
		store.subscribers[uuid] = updated
		store.mu.Unlock()
		if !found {
			writeError(w, http.StatusNotFound, "subscriber not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
