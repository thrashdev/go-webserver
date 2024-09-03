package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"internal/database"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type apiConfig struct {
	fileServerHits int
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		cfg.fileServerHits++
	}

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

func (cfg *apiConfig) handlerResetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg.fileServerHits = 0
}

func (cfg *apiConfig) handlerAdminMetrics(w http.ResponseWriter, r *http.Request) {
	template_html := `<html>

<body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
</body>

</html>`

	res := fmt.Sprintf(template_html, cfg.fileServerHits)
	w.Write([]byte(res))
	w.WriteHeader(200)
	w.Header().Add("Content-Type", "text/html")
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	w.Write([]byte(msg))
	return
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	resp, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return errors.New(fmt.Sprintf("Couldn't marshall json %v", payload))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(resp)
	return nil
}

func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

func censor_words(source string, wordsToReplace []string) string {
	replaced := []string{}
	replacement := "****"
	words := strings.Split(source, " ")
	for _, word := range words {
		if contains(wordsToReplace, strings.ToLower(word)) {
			replaced = append(replaced, replacement)
		} else {
			replaced = append(replaced, word)
		}
	}
	result := strings.Join(replaced, " ")
	return result
}

func handlerPOSTChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		err_msg := fmt.Sprintf("Error decoding parameters %v ", err)
		log.Printf(err_msg)
		respondWithError(w, 500, err_msg)
		return
	}
	if len(params.Body) > 140 {
		type returnServerError struct {
			Error string `json:"error"`
		}
		errMsg := returnServerError{Error: "Chirp is too long"}
		respondWithJSON(w, 400, errMsg)
		return
		// fmt.Println(errMsg.error)
	}
	profanities := []string{"kerfuffle", "sharbert", "fornax"}
	cleaned_body := censor_words(params.Body, profanities)

	type returnVals struct {
		CleanedBody string `json:"cleaned_body"`
	}
	rVals := returnVals{CleanedBody: cleaned_body}
	respondWithJSON(w, 200, rVals)
	return
}

func main() {
	serveMux := http.NewServeMux()
	apiConf := apiConfig{}
	serveMux.Handle("/app/", http.StripPrefix("/app/", apiConf.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	serveMux.HandleFunc("GET /api/healthz/", handlerReadiness)
	serveMux.HandleFunc("GET /api/metrics/", apiConf.handlerMetrics)
	serveMux.HandleFunc("/api/reset/", apiConf.handlerResetMetrics)
	serveMux.HandleFunc("/admin/metrics", apiConf.handlerAdminMetrics)
	serveMux.HandleFunc("POST /api/chirps", handlerPOSTChirp)
	server := http.Server{Handler: serveMux, Addr: ":8080"}
	server.ListenAndServe()
}
