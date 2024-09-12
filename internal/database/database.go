package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type Chirp struct {
	Id       int    `json:"id"`
	Body     string `json:"body"`
	AuthorID int    `json:"author_id"`
}

type User struct {
	Id          int    `json:"id"`
	Email       string `json:"email"`
	Password    []byte
	IsChirpyRed bool `json:"is_chirpy_red"`
}

type UserDTO struct {
	Id          int    `json:"id"`
	Email       string `json:"email"`
	IsChirpyRed bool   `json:"is_chirpy_red"`
}

type DBStructure struct {
	Chirps        map[int]Chirp           `json:"chirps"`
	Users         map[int]User            `json:"users"`
	RefreshTokens map[string]RefreshToken `json:"refresh_tokens"`
}

type RefreshToken struct {
	UserID    int       `json:"user_id"`
	Token     string    `json"refresh_token:"`
	CreatedAt time.Time `json:"created_at"`
}

func NewDB(path string) (*DB, error) {
	defaultStructure, err := json.Marshal(DBStructure{Chirps: make(map[int]Chirp), Users: make(map[int]User), RefreshTokens: make(map[string]RefreshToken)})
	if err != nil {
		debug.PrintStack()
		log.Fatal()
	}
	_, err = os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			os.WriteFile(path, defaultStructure, 0666)
		} else {
			log.Fatal(err)
		}
	}
	mux := sync.RWMutex{}
	db := DB{path: path, mux: &mux}

	return &db, nil
}

func (db *DB) loadDB() (DBStructure, error) {
	err := db.ensureDB()
	if err != nil {
		log.Fatal(err)
	}

	db.mux.RLock()
	defer db.mux.RUnlock()
	rawFileData, err := os.ReadFile(db.path)
	if err != nil {
		log.Fatal(err)
	}
	dbStructure := DBStructure{}
	json.Unmarshal(rawFileData, &dbStructure)
	return dbStructure, nil
}

func (db *DB) writeDB(dbStructure DBStructure) error {
	marshalledDB, err := json.Marshal(dbStructure)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	db.mux.Lock()
	defer db.mux.Unlock()
	err = os.WriteFile(db.path, marshalledDB, 0666)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (db *DB) ensureDB() error {
	_, err := os.Stat(db.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			NewDB(db.path)
		} else {
			log.Fatal(err)
		}
	}
	return nil
}

func (dbStructure *DBStructure) getMaxChirpID() int {
	maxID := 0
	for key, _ := range dbStructure.Chirps {
		if key > maxID {
			maxID = key
		}
	}
	return maxID
}

func (dbStructure *DBStructure) getMaxUserID() int {
	maxID := 0
	for key, _ := range dbStructure.Users {
		if key > maxID {
			maxID = key
		}
	}
	return maxID
}

func (db *DB) CreateChirp(body string, userID int) (Chirp, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
	}
	id := dbStructure.getMaxChirpID() + 1
	chirp := Chirp{Id: id, Body: body, AuthorID: userID}
	dbStructure.Chirps[id] = chirp
	err = db.writeDB(dbStructure)
	if err != nil {
		log.Fatal(err)
	}
	return chirp, nil

}

func (db *DB) GetChirps() ([]Chirp, error) {
	chirps := []Chirp{}
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range dbStructure.Chirps {
		chirps = append(chirps, v)
	}
	return chirps, nil
}

func (db *DB) GetChirpByID(chirpID int) (Chirp, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	chirp, ok := dbStructure.Chirps[chirpID]
	if ok {
		return chirp, nil
	}
	return Chirp{}, errors.New("Chirp not found")

}

func (db *DB) DeleteChirpByID(chirpID int) error {
	dbStructure, err := db.loadDB()
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	delete(dbStructure.Chirps, chirpID)
	err = db.writeDB(dbStructure)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	return nil
}

func (db *DB) CreateUser(email string, password string) (User, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.GetUserByEmail(email)
	if err == nil {
		return User{}, errors.New("User with this email already exists")
	}
	id := dbStructure.getMaxUserID() + 1
	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		debug.PrintStack()
		log.Fatal()
	}
	user := User{Id: id, Email: email, Password: hashedPass}
	dbStructure.Users[id] = user
	err = db.writeDB(dbStructure)
	if err != nil {
		log.Fatal(err)
	}
	return user, nil
}

func (db *DB) GetUsers() ([]User, error) {
	users := []User{}
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range dbStructure.Users {
		users = append(users, v)
	}
	return users, nil
}

func (db *DB) GetUserByID(userID int) (User, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	user, ok := dbStructure.Users[userID]
	if ok {
		return user, nil
	}
	return User{}, errors.New("User not found")

}

func (db *DB) GetUserByEmail(email string) (User, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
	for _, v := range dbStructure.Users {
		if v.Email == email {
			return v, nil
		}
	}
	return User{}, errors.New("User not found")
}

func (db *DB) UpdateUser(user User) (User, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		fmt.Println(err)
		return User{}, errors.New("Couldn't load db")
	}
	dbStructure.Users[user.Id] = user
	newUser := dbStructure.Users[user.Id]
	db.writeDB(dbStructure)
	return newUser, nil
}

func (db *DB) CreateRefreshToken(rToken RefreshToken) (RefreshToken, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		fmt.Println(err)
		return RefreshToken{}, errors.New("Couldn't load db")
	}
	dbStructure.RefreshTokens[rToken.Token] = rToken
	db.writeDB(dbStructure)
	return rToken, nil
}

func (db *DB) GetRefreshToken(token string) (RefreshToken, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal("Couldn't load db")
	}
	if rToken, ok := dbStructure.RefreshTokens[token]; ok {
		return rToken, nil
	}
	return RefreshToken{}, errors.New("Refresh token not found")

}

func (db *DB) DeleteRefreshToken(token string) error {
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal("Couldn't load db")
	}
	delete(dbStructure.RefreshTokens, token)
	db.writeDB(dbStructure)
	return nil

}
