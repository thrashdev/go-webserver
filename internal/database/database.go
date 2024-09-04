package database

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type Chirp struct {
	id   int
	body string
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chrirps"`
}

func NewDB(path string) (*DB, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		os.WriteFile(path, []byte(""), 0666)
	} else {
		log.Fatal(err)
	}
	mux := sync.RWMutex{}
	db := DB{path: path, mux: &mux}

	return &db, nil
}

func (db *DB) loadDB() (DBStructure, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	rawFileData, err := os.ReadFile(db.path)
	if err != nil {
		log.Fatal(err)
	}
	unmarshalledMap := map[int]Chirp{}
	json.Unmarshal(rawFileData, &unmarshalledMap)
	return DBStructure{Chirps: unmarshalledMap}, nil
}

func (db *DB) writeDB(dbStructure DBStructure) error {
	marshalledMap, err := json.Marshal(dbStructure.Chirps)
	if err != nil {
		log.Fatal(err)
	}
	db.mux.Lock()
	err = os.WriteFile(db.path, marshalledMap, 0666)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (db *DB) ensureDB() error {
	_, err := os.Stat(db.path)
	if errors.Is(err, os.ErrNotExist) {
		os.WriteFile(db.path, []byte(""), 0666)
	} else {
		log.Fatal(err)
	}
	return nil
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	dbStructure = 
}

func (dbStructure *DBStructure) getMaxID() (int, error) {
	maxID := 0
	for key, _ := range dbStructure.Chirps {
		if key > maxID {
			maxID = key
		}
	}
	return maxID, nil
}
