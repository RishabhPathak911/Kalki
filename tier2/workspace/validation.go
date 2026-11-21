package workspace

import (
	"encoding/json"
	"net/http"
	"os"
)

func validation(w http.ResponseWriter) {
	b, err := os.ReadFile("data/validation-report.json")
	if err != nil {
		respond(w, 200, map[string]string{"status": "pending"})
		return
	}
	if len(b) > 2<<20 || !json.Valid(b) {
		failure(w, 503)
		return
	}
	w.Write(b)
}
