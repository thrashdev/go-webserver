package main

import (
	"net/http"
	"strconv"
)

type apiConfig struct {
	fileServerHits int
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	cfg.fileServerHits++
	return next
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("OK"))
	w.Header().Set("Content-Type", "text-plain")
	w.Header().Set("charset", "utf-8")

}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	hits := "Hits: " + strconv.Itoa(cfg.fileServerHits)
	w.WriteHeader(200)
	w.Write([]byte(hits))
}

func main() {
	serveMux := http.NewServeMux()
	apiConf := apiConfig{}
	serveMux.Handle("/app/", http.StripPrefix("/app/", apiConf.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	serveMux.HandleFunc("/healthz/", handlerReadiness)
	serveMux.HandleFunc("/metrics/", apiConf.handlerMetrics)
	server := http.Server{Handler: serveMux, Addr: ":8080"}
	server.ListenAndServe()
}
