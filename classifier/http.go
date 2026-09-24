package classifier

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func Handler(service *Classifier) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/classify", func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			URL string `json:"url"`
		}
		decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if input.URL == "" {
			writeError(writer, http.StatusBadRequest, errors.New("url is required"))
			return
		}
		result, err := service.ClassifyURL(request.Context(), input.URL)
		if err != nil {
			writeError(writer, http.StatusUnprocessableEntity, err)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	return mux
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}
