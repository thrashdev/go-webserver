package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"internal/database"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
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

func newAccessToken(u database.User) (token string) {
	jwtSecret := os.Getenv("JWT_SECRET")
	expirationTime := time.Now().Add(time.Hour * time.Duration(1))
	claims := jwt.RegisteredClaims{Issuer: "chirpy", IssuedAt: jwt.NewNumericDate(time.Now().UTC()), Subject: string(u.Id), ExpiresAt: jwt.NewNumericDate(expirationTime)}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := tokenObj.SignedString([]byte(jwtSecret))
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	return accessToken
}

func authenticateJWT(r *http.Request) (*jwt.Token, error) {
	tokenHeader := r.Header.Get("Authorization")
	tokenString := strings.Replace(tokenHeader, "Bearer ", "", 1)
	claims := jwt.RegisteredClaims{}
	tokenObj, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		return &jwt.Token{}, err
	}
	return tokenObj, nil
}

func getUserIdFromJwtToken(token *jwt.Token) (userID int, err error) {
	idString, err := token.Claims.GetSubject()
	if err != nil {
		log.Fatal(err)
	}
	// var userID int
	idBytes := []byte(idString)
	userID = int(idBytes[0])
	return userID, nil

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
	}
	profanities := []string{"kerfuffle", "sharbert", "fornax"}
	cleaned_body := censor_words(params.Body, profanities)

	db, err := database.NewDB("database.json")
	if err != nil {
		log.Fatal(err)
	}
	token, err := authenticateJWT(r)
	if err != nil {
		log.Println(err)
		w.Write([]byte(err.Error()))
		w.WriteHeader(401)
		return
	}
	stringUserID, err := token.Claims.GetSubject()
	if err != nil {
		log.Println(err)
		w.Write([]byte(err.Error()))
		w.WriteHeader(401)
		return
	}
	userID := int([]byte(stringUserID)[0])
	chirp, err := db.CreateChirp(cleaned_body, userID)
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
	authorIDString := r.URL.Query().Get("author_id")
	sortTypeString := r.URL.Query().Get("sort")
	sortType := "asc"
	if sortTypeString == "desc" {
		sortType = "desc"
	}

	chirps, err := db.GetChirps()
	if err != nil {
		log.Fatal(err)
	}
	if authorIDString != "" {
		newChirps := []database.Chirp{}
		authorID, err := strconv.Atoi(authorIDString)
		if err != nil {
			log.Fatal(err)
			return
		}
		for _, chirp := range chirps {
			if chirp.AuthorID == authorID {
				newChirps = append(newChirps, chirp)
			}
		}
		chirps = newChirps
	}
	//asc
	switch sortType {
	case "asc":
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].Id < chirps[j].Id
		})
	case "desc":
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].Id > chirps[j].Id
		})
	}

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
		database.UserDTO
	}
	userDto := database.UserDTO{Id: user.Id, Email: user.Email, IsChirpyRed: user.IsChirpyRed}
	respJSON := responseJSON{UserDTO: userDto}
	err = respondWithJSON(w, 201, respJSON)

}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		// ExpirationTime int    `json:"expires_in_seconds,omitempty"`
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
		// log.Println(err)
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "text-plain")
		w.Header().Set("charset", "utf-8")
		w.Write([]byte(err.Error() + "\n"))
		return
	}
	err = bcrypt.CompareHashAndPassword(user.Password, []byte(params.Password))
	if err != nil {
		w.WriteHeader(401)
		hashedPass, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
		if err != nil {
			debug.PrintStack()
			log.Fatal()
		}
		log.Println("Password validation error", err)
		log.Println("Request password: ", params.Password)
		log.Println("User password: ", user.Password)
		log.Println("Hashed request password: ", hashedPass)
		return
	}
	accessToken := newAccessToken(user)
	refreshTokenSource := make([]byte, 10)
	_, err = rand.Read(refreshTokenSource)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	refreshTokenString := hex.EncodeToString(refreshTokenSource)
	rToken := database.RefreshToken{UserID: user.Id, Token: refreshTokenString, CreatedAt: time.Now()}
	db.CreateRefreshToken(rToken)
	type responseJSON struct {
		database.UserDTO
		AccessToken  string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	userDto := database.UserDTO{Id: user.Id, Email: user.Email, IsChirpyRed: user.IsChirpyRed}
	respJSON, err := json.Marshal(responseJSON{UserDTO: userDto, AccessToken: accessToken, RefreshToken: refreshTokenString})
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	w.WriteHeader(200)
	w.Write(respJSON)

}

func handlePUTUser(w http.ResponseWriter, r *http.Request) {
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
	tokenHeader := r.Header.Get("Authorization")
	tokenString := strings.Replace(tokenHeader, "Bearer ", "", 1)
	claims := jwt.RegisteredClaims{}
	tokenObj, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(401)
		return
	}
	stringUserID, err := tokenObj.Claims.GetSubject()
	if err != nil {
		fmt.Println(err)
		return
	}
	userID := int([]byte(stringUserID)[0])
	db, err := database.NewDB("database.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	dbUser, err := db.GetUserByID(int(userID))
	if err != nil {
		fmt.Println(err)
		return
	}
	user := database.User{Id: dbUser.Id, Email: params.Email, Password: []byte(params.Password)}
	newUser, err := db.UpdateUser(user)
	if err != nil {
		fmt.Println(err)
		return
	}
	type responseJSON struct {
		Id    int    `json:"id"`
		Email string `json:"email"`
	}
	respJSON := responseJSON{Id: newUser.Id, Email: newUser.Email}
	respondWithJSON(w, 200, respJSON)

	// fmt.Println(token)

}

func handlePOSTRefresh(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Executing refresh")
	db, err := database.NewDB("database.json")
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	tokenHeader := r.Header.Get("Authorization")
	tokenString := strings.Replace(tokenHeader, "Bearer ", "", 1)
	token, err := db.GetRefreshToken(tokenString)
	if err != nil {
		w.WriteHeader(401)
		return
	}
	now := time.Now()
	//if more than 60 days then it's expired
	fmt.Println("Time-diff: ", now.Sub(token.CreatedAt))
	if now.Sub(token.CreatedAt) > time.Duration(1)*time.Hour*24*60 {
		w.WriteHeader(401)
		return
	}
	userID := token.UserID
	user, err := db.GetUserByID(userID)
	if err != nil {
		w.WriteHeader(400)
		return
	}

	accessToken := newAccessToken(user)
	type responseJSON struct {
		Token string `json:"token"`
	}
	respJSON := responseJSON{Token: accessToken}
	respondWithJSON(w, 200, respJSON)

}

func handlePOSTRevoke(w http.ResponseWriter, r *http.Request) {
	db, err := database.NewDB("database.json")
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	tokenHeader := r.Header.Get("Authorization")
	tokenString := strings.Replace(tokenHeader, "Bearer ", "", 1)
	token, err := db.GetRefreshToken(tokenString)
	if err != nil {
		respondWithError(w, 401, "No token in db")
		return
	}
	now := time.Now()
	//if more than 60 days then it's expired
	fmt.Println("Time-diff: ", now.Sub(token.CreatedAt))
	if now.Sub(token.CreatedAt) > time.Duration(1)*time.Hour*24*60 {
		respondWithError(w, 401, "Token expired")
		return
	}
	db.DeleteRefreshToken(tokenString)
	w.WriteHeader(204)
	return

}

func handleDELETEChirpByID(w http.ResponseWriter, r *http.Request) {
	db, err := database.NewDB("database.json")
	if err != nil {
		log.Fatal()
	}
	token, err := authenticateJWT(r)
	if err != nil {
		respondWithError(w, 403, err.Error())
		return
	}
	fmt.Println(token)
	userID, err := getUserIdFromJwtToken(token)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(userID)
	chirpID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	chirp, err := db.GetChirpByID(chirpID)
	if err != nil {
		log.Println(err)
		return
	}
	if chirp.AuthorID != userID {
		w.WriteHeader(403)
		return
	}
	err = db.DeleteChirpByID(chirpID)
	w.WriteHeader(204)
	return

}

func handlePolkaWebhook(w http.ResponseWriter, r *http.Request) {
	type requestData struct {
		UserID int `json:"user_id"`
	}
	type parameters struct {
		Event string      `json:"event"`
		Data  requestData `json:"data"`
	}
	apiKeyHeader := r.Header.Get("Authorization")
	if !strings.Contains(apiKeyHeader, "ApiKey") {
		w.WriteHeader(401)
		return
	}
	apiKeyRequest := strings.Replace(apiKeyHeader, "ApiKey ", "", 1)
	apiKeySecret := os.Getenv("POLKA_API_KEY")
	if apiKeyRequest != apiKeySecret {
		w.WriteHeader(401)
		return
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
	if params.Event != "user.upgraded" {
		w.WriteHeader(204)
		return
	}
	userID := params.Data.UserID
	db, err := database.NewDB("database.json")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	user, err := db.GetUserByID(userID)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
		return
	}
	user.IsChirpyRed = true
	db.UpdateUser(user)
	w.WriteHeader(204)
	return
}

func main() {
	serveMux := http.NewServeMux()
	apiConf := apiConfig{}
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
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
	serveMux.HandleFunc("PUT /api/users", handlePUTUser)
	serveMux.HandleFunc("POST /api/refresh", handlePOSTRefresh)
	serveMux.HandleFunc("POST /api/revoke", handlePOSTRevoke)
	serveMux.HandleFunc("DELETE /api/chirps/{id}", handleDELETEChirpByID)
	serveMux.HandleFunc("POST /api/polka/webhooks", handlePolkaWebhook)
	server := http.Server{Handler: serveMux, Addr: ":8080"}
	server.ListenAndServe()
}
