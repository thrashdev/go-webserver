package database

import "sync"

type DB struct {
	path string
	mux  *sync.RWMutex
}

type Chirp struct {
	id   int32
	body string
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chrirps"`
}
