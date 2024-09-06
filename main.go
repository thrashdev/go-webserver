package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"internal/database"
	"log"
	"net/http"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
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

func handleReadiness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("OK"))
	w.Header().Set("Content-Type", "text-plain")
	w.Header().Set("charset", "utf-8")

}

func (cfg *apiConfig) handleMetrics(w http.ResponseWriter, r *http.Request) {
	hits := "Hits: " + strconv.Itoa(cfg.fileServerHits)
	w.WriteHeader(200)
	w.Write([]byte(hits))
}

func (cfg *apiConfig) handleResetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg.fileServerHits = 0
}

func (cfg *apiConfig) handleAdminMetrics(w http.ResponseWriter, r *http.Request) {
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

func handlePOSTChirp(w http.ResponseWriter, r *http.Request) {
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

	db, err := database.NewDB("database.json")
	if err != nil {
		log.Fatal(err)
	}
	chirp, err := db.CreateChirp(cleaned_body)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	respondWithJSON(w, 201, chirp)
	return
}

func handleGETChirps(w http.ResponseWriter, r *http.Request) {
	db, err := database.NewDB("database.json")
	if err != nil {
		log.Fatal(err)
	}

	chirps, err := db.GetChirps()
	if err != nil {
		log.Fatal(err)
	}
	sort.Slice(chirps, func(i, j int) bool {
		return chirps[i].Id < chirps[j].Id
	})

	jsonResp, err := json.Marshal(chirps)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}

	w.WriteHeader(200)
	w.Write(jsonResp)

}

func handleGETChirpByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	db, err := database.NewDB("database.json")
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	chirp, err := db.GetChirpByID(id)
	if err != nil {
		w.WriteHeader(404)
		return
	}
	jsonResp, err := json.Marshal(chirp)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	w.WriteHeader(200)
	w.Write(jsonResp)

}

func handlePOSTUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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
	db, err := database.NewDB("database.json")
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	user, err := db.CreateUser(params.Email, params.Password)
	if err != nil {
		debug.PrintStack()
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}
	type responseJSON struct {
		Id    int    `json:"id"`
		Email string `json:"email"`
	}
	respJSON := responseJSON{Id: user.Id, Email: user.Email}
	err = respondWithJSON(w, 201, respJSON)

}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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
	db, err := database.NewDB("database.json")
	if err != nil {
		debug.PrintStack()
		log.Fatal()
	}
	user, err := db.GetUserByEmail(params.Email)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	err = bcrypt.CompareHashAndPassword(user.Password, []byte(params.Password))
	if err != nil {
		w.WriteHeader(401)
		return
	}
	type responseJSON struct {
		Id    int    `json:"id"`
		Email string `json:"email"`
	}
	respJSON, err := json.Marshal(responseJSON{Id: user.Id, Email: user.Email})
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	w.WriteHeader(200)
	w.Write(respJSON)

}

func main() {
	serveMux := http.NewServeMux()
	apiConf := apiConfig{}
	serveMux.Handle("/app/", http.StripPrefix("/app/", apiConf.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	serveMux.HandleFunc("GET /api/healthz/", handleReadiness)
	serveMux.HandleFunc("GET /api/metrics/", apiConf.handleMetrics)
	serveMux.HandleFunc("/api/reset/", apiConf.handleResetMetrics)
	serveMux.HandleFunc("/admin/metrics", apiConf.handleAdminMetrics)
	serveMux.HandleFunc("POST /api/chirps", handlePOSTChirp)
	serveMux.HandleFunc("GET /api/chirps", handleGETChirps)
	serveMux.HandleFunc("GET /api/chirps/{id}", handleGETChirpByID)
	serveMux.HandleFunc("POST /api/users", handlePOSTUser)
	serveMux.HandleFunc("POST /api/login", handleLogin)
	server := http.Server{Handler: serveMux, Addr: ":8080"}
	server.ListenAndServe()
}
