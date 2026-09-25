package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/solve", handleSolve)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "ok\n")
	})
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("circuit 求解服务监听 :8080")
	log.Fatal(srv.ListenAndServe())
}

type errorResponse struct {
	Error      string     `json:"error"`
	Components [][]string `json:"components,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func handleSolve(w http.ResponseWriter, r *http.Request) {
	var req solveRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields() // 多余字段 → 整份拒绝
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求 JSON 无效：" + err.Error()})
		return
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求体在 JSON 之后还有多余内容"})
		return
	}
	result, aerr := analyze(&req)
	if aerr != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: aerr.msg, Components: aerr.components})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
