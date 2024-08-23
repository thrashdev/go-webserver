package main

import (
	"net/http"
)

type apiConfig struct {
	fileServerHits int
}

func (cfg *apiConfig) middlewareMetricsInt(next http.Handler) http.Handler {

}

func handleReadiness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("OK"))
	w.Header().Set("Content-Type", "text-plain")
	w.Header().Set("charset", "utf-8")

}

func main() {
	serveMux := http.NewServeMux()
	serveMux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir("."))))
	serveMux.HandleFunc("/healthz/", handleReadiness)
	server := http.Server{Handler: serveMux, Addr: ":8080"}
	server.ListenAndServe()
}
