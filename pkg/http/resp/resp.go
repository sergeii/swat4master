package resp

import (
	"encoding/json"
	"net/http"
)

func JSONResponse(jsonValue interface{}, w http.ResponseWriter, status int) {
	result, err := json.Marshal(jsonValue)
	// Не удалось серилизовать json по некой очень редкой проблеме
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(result) // nolint:errcheck
}
