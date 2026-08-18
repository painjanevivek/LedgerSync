package httptransport

import (
	"encoding/json"
	"net/http"
)

type DependencyCheck func() error

func NewHealthHandler(ready DependencyCheck) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeStatus(writer, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, _ *http.Request) {
		if ready != nil {
			if err := ready(); err != nil {
				writeStatus(writer, http.StatusServiceUnavailable, "not_ready")
				return
			}
		}
		writeStatus(writer, http.StatusOK, "ready")
	})
	return mux
}

func writeStatus(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"status": value})
}
