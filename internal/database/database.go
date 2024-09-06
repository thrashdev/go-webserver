package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type Chirp struct {
	Id   int    `json:"id"`
	Body string `json:"body"`
}

type User struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chirps"`
	Users  map[int]User  `json:"users"`
}

func NewDB(path string) (*DB, error) {
	defaultStructure, err := json.Marshal(DBStructure{Chirps: make(map[int]Chirp), Users: make(map[int]User)})
	if err != nil {
		debug.PrintStack()
		log.Fatal()
	}
	fmt.Println("Default structure: ", string(defaultStructure))
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

func (db *DB) CreateChirp(body string) (Chirp, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
	}
	id := dbStructure.getMaxChirpID() + 1
	chirp := Chirp{Id: id, Body: body}
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

func (db *DB) CreateUser(email string) (User, error) {
	dbStructure, err := db.loadDB()
	fmt.Println(dbStructure)
	if err != nil {
		log.Fatal(err)
	}
	id := dbStructure.getMaxUserID() + 1
	user := User{Id: id, Email: email}
	dbStructure.Users[id] = user
	err = db.writeDB(dbStructure)
	if err != nil {
		log.Fatal(err)
	}
	return user, nil
}
