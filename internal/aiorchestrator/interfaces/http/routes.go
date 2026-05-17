package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"flow-anything/internal/aiorchestrator/application"
	"flow-anything/internal/aiorchestrator/ports"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/runtimeevent"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/tenant"
)

func RegisterRoutes(mux *http.ServeMux, app *application.Service, subscribers ...ports.RuntimeEventSubscriber) {
	mux.HandleFunc("POST /v1/events", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req event.Event
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid event json",
				},
			})
			return
		}

		resp, err := app.HandleEvent(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	if len(subscribers) > 0 && subscribers[0] != nil {
		registerLiveEventRoutes(mux, subscribers[0])
	}

	mux.HandleFunc("GET /v1/traces/{trace_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		traceID := r.PathValue("trace_id")

		resp, err := app.GetTrace(r.Context(), tenantID, traceID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})
}

func registerLiveEventRoutes(mux *http.ServeMux, subscriber ports.RuntimeEventSubscriber) {
	mux.HandleFunc("GET /v1/live-events/{trace_id}", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			httpserver.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{
					"code":    "streaming_not_supported",
					"message": "http streaming is not supported",
				},
			})
			return
		}
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		traceID := r.PathValue("trace_id")
		if tenantID.Empty() || traceID == "" {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_argument",
					"message": "tenant_id and trace_id are required",
				},
			})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		events, cancel := subscriber.Subscribe(r.Context(), tenantID, traceID)
		defer cancel()

		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		writeRuntimeSSE(w, flusher, "connected", runtimeevent.Event{
			Type:      "connected",
			TenantID:  tenantID,
			TraceID:   traceID,
			CreatedAt: time.Now().UTC(),
		})

		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				_, _ = w.Write([]byte(": heartbeat\n\n"))
				flusher.Flush()
			case evt := <-events:
				writeRuntimeSSE(w, flusher, string(evt.Type), evt)
			}
		}
	})
}

func writeRuntimeSSE(w http.ResponseWriter, flusher http.Flusher, eventName string, event runtimeevent.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("event: " + eventName + "\n"))
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n\n"))
	flusher.Flush()
}
