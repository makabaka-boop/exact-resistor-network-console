package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/solve", handleSolve)
	mux.HandleFunc("/api/health", handleHealth)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           cors(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("circuit solver listening on :8080")
	log.Fatal(srv.ListenAndServe())
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "up"})
}

func handleSolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, SolveResponse{
			Status:  StatusError,
			Message: "method not allowed: use POST",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // 多余字段：整份拒绝

	var req SolveRequest
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SolveResponse{
			Status:  StatusError,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}
	if dec.More() {
		writeJSON(w, http.StatusBadRequest, SolveResponse{
			Status:  StatusError,
			Message: "unexpected data after JSON body",
		})
		return
	}

	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, SolveResponse{
			Status:  StatusError,
			Message: err.Error(),
		})
		return
	}

	resp := solve(&req)
	code := http.StatusOK
	if resp.Status == StatusError {
		code = http.StatusBadRequest
	}
	writeJSON(w, code, resp)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
