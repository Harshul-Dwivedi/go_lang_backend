package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
)

type Note struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// for memory storage of notes like key, value pairs
var notes = make(map[int]Note)

// mutex ensure only one goroutine access the notes map at a time
var mu sync.Mutex

// create a new note (for POST request)
// In GO every handler must have these 2 args
// responseWriter -> to write response back to client
// request -> represents all incoming request from client
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

	//headers describe that response is in json , not plain text
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// get all notes (for GET request)
func getNotesHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	// convert map into slice of notes
	// maps can't be directly converted to json arrays so we use slice
	notesList := make([]Note, 0, len(notes))
	for _, n := range notes {
		notesList = append(notesList, n)
	}
	mu.Unlock()

	//send all notes as json response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notesList)
}

// get note by id
func getNoteHandler(w http.ResponseWriter, r *http.Request) {
	// mux.Vars returns map of path params (like /notes/{id})
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"]) // convert string id to int because our notes map uses 'int' keys
	if err != nil {
		http.Error(w, "Invalid note id", http.StatusBadRequest)
		return
	}
	mu.Lock()
	note, exists := notes[id]
	mu.Unlock()

	if !exists {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// delete note by id
func deleteNoteHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid note id", http.StatusBadRequest)
		return
	}
	// lock and delete if exists
	mu.Lock()
	_, exists := notes[id]
	if exists {
		delete(notes, id)
	}
	mu.Unlock()

	if !exists {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}

	//return empty resposne with status 204 (no content)
	w.WriteHeader(http.StatusNoContent)
}

// MAIN Function
func main() {
	// create new router
	// router is responsible for matching incoming req to correct handler
	r := mux.NewRouter()
	r.HandleFunc("/notes", createNewNoteHandler).Methods("POST")     // create new note
	r.HandleFunc("/notes", getNotesHandler).Methods("GET")           // get all notes
	r.HandleFunc("/notes/{id}", getNoteHandler).Methods("GET")       // get note by ID
	r.HandleFunc("/notes/{id}", deleteNoteHandler).Methods("DELETE") // delete note by ID

	//start server
	fmt.Println("Server running on local host: 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
