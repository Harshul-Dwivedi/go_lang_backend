package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// global db connection
// sql db is safe for concurrent use so we dont need mutex
var db *sql.DB

// initialize sql db and table
func initDB() {
	var err error
	// create notes.db file
	db, err = sql.Open("sqlite3", "./notes.db")
	if err != nil {
		log.Fatal(err)
	}
	// create notes table if not exists
	createTable := `
	CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL
	);`
	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}
}

type Note struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

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
	// insert into db
	// using '?' placeholder helps prevent sql injection
	// by using placeholders, query treats user input as data and not sql code
	res, err := db.Exec("INSERT INTO notes (title, content) VALUES (?, ?)", note.Title, note.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	note.ID = int(id)

	//headers describe that response is in json , not plain text
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// get all notes (for GET request)
func getNotesHandler(w http.ResponseWriter, r *http.Request) {
	// SQL query to fetch all rows
	rows, err := db.Query("SELECT id, title, content FROM notes")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	//defer to ensure we release db resources once done
	defer rows.Close()
	var notesList []Note
	for rows.Next() {
		var n Note
		err := rows.Scan(&n.ID, &n.Title, &n.Content)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		notesList = append(notesList, n)
	}

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
	var note Note
	err = db.QueryRow("SELECT id, title, content FROM notes WHERE id = ?", id).Scan(&note.ID, &note.Title, &note.Content)
	if err == sql.ErrNoRows {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	_, err = db.Exec("DELETE FROM notes WHERE id=?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//return empty resposne with status 204 (no content)
	w.WriteHeader(http.StatusNoContent)
}

// update note by id
func updateNoteHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid note id", http.StatusBadRequest)
		return
	}
	var updatedData Note
	//Reads json from request body and fills updatedData
	err = json.NewDecoder(r.Body).Decode(&updatedData)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	_, err = db.Exec("UPDATE notes SET title=?, content=? WHERE id=?", updatedData.Title, updatedData.Content, updatedData.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updatedData.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedData)
}

// MAIN Function
func main() {
	initDB()
	// create new router
	// router is responsible for matching incoming req to correct handler
	r := mux.NewRouter()
	r.HandleFunc("/notes", createNewNoteHandler).Methods("POST")     // create new note
	r.HandleFunc("/notes", getNotesHandler).Methods("GET")           // get all notes
	r.HandleFunc("/notes/{id}", getNoteHandler).Methods("GET")       // get note by ID
	r.HandleFunc("/notes/{id}", deleteNoteHandler).Methods("DELETE") // delete note by ID
	r.HandleFunc("/notes/{id}", updateNoteHandler).Methods("PUT")    // update note by ID
	//start server
	fmt.Println("Server running on local host: 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
