// Copyright (c) 2026 Develeap
// SPDX-License-Identifier: MIT

package mockserver

import (
	"net/http"

	hyperping "github.com/hyperping/hyperping-go"
)

func registerReportHandlers(mux *http.ServeMux, store *mockStore) {
	mux.HandleFunc("GET /v2/reporting/monitor-reports", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		list := make([]hyperping.MonitorReport, 0, len(store.reports))
		for _, rep := range store.reports {
			list = append(list, *rep)
		}
		store.mu.RUnlock()
		writeJSON(w, http.StatusOK, hyperping.ListMonitorReportsResponse{
			Monitors: list,
		})
	})

	mux.HandleFunc("POST /v2/reporting/monitor-reports", func(w http.ResponseWriter, r *http.Request) {
		var rep hyperping.MonitorReport
		if err := decodeBody(r, &rep); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if rep.UUID == "" {
			rep.UUID = newUUID()
		}
		store.mu.Lock()
		store.reports[rep.UUID] = &rep
		store.mu.Unlock()
		writeJSON(w, http.StatusCreated, rep)
	})

	mux.HandleFunc("GET /v2/reporting/monitor-reports/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		store.mu.RLock()
		rep, ok := store.reports[uuid]
		store.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "report not found")
			return
		}
		writeJSON(w, http.StatusOK, *rep)
	})
}
