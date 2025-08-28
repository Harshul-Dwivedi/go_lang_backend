package main

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"sync"
)

type Note struct {
	ID      int    `json: "id"`
	Title   string `json: "title"`
	Content string `json: "content"`
}

// for memory storage of notes like key, value pairs
var notes = make(map[int]Note)

// mutex ensure only one goroutine access the notes map at a time
var mu sync.Mutex

// create a new note
func createNewNoteHandler(w http.ResponseWriter, r *http.Request) {
	var note Note
	// decode json from request body into struct
	err := json.NewDecoder(r.Body).Decode(&note)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	mu.Lock()
	note.ID = rand.IntN(100000)
	notes[note.ID] = note //save note into map
	mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}
