package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// ========== MODELS ============//
type Note struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	UserID  int    `json:"user_id"`
}

// represents registered user
// json tag '-' means we dont expose it in api
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ============GLOBALS==========//
var db *sql.DB
var jwtKey = []byte("my_secret_key") // secret key for signing tokens

// structure of jwt
type Claims struct {
	UserId int `json:"user_id"`
	jwt.StandardClaims
}

// signup new user
func signupHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	json.NewDecoder(r.Body).Decode(&user)

	// Hash the plain password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	// Insert into database
	_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", user.Username, string(hashedPassword))
	if err != nil {
		http.Error(w, "Error creating user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User created successfully"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var creds User
	json.NewDecoder(r.Body).Decode(&creds)

	// Fetch user from DB
	var dbUser User
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", creds.Username).
		Scan(&dbUser.ID, &dbUser.Password) // dbUser.Password will actually hold the hashed password
	if err != nil {
		http.Error(w, "Invalid Username", http.StatusUnauthorized)
		return
	}

	// Compare hash from DB with plain password from request
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.Password), []byte(creds.Password))
	if err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		UserId: dbUser.ID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, "Could not generate token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

// Middleware to protect routes
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			http.Error(w, "Missing Token", http.StatusUnauthorized)
			return
		}
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid Token", http.StatusUnauthorized)
			return
		}
		// store user id
		r.Header.Set("userId", fmt.Sprint(claims.UserId))
		next.ServeHTTP(w, r)
	})
}

func createNoteHandler(w http.ResponseWriter, r *http.Request) {
	var note Note
	json.NewDecoder(r.Body).Decode(&note)
	// get user id from req header set in middleware
	userId := r.Header.Get("userId")
	_, err := db.Exec("INSERT INTO notes (title, content, user_id) VALUES (?, ?, ?)", note.Title, note.Content, userId)
	if err != nil {
		http.Error(w, "Error saving note", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Note created"})
}

func getNotesHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("userId")
	rows, err := db.Query("SELECT id, title, content, user_id FROM notes WHERE user_id = ?", userId)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var notes []Note
	for rows.Next() {
		var note Note
		rows.Scan(&note.ID, &note.Title, &note.Content, &note.UserID)
		notes = append(notes, note)
	}
	json.NewEncoder(w).Encode(notes)
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./notes.db")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT,
			content TEXT,
			user_id INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	//Router
	r := mux.NewRouter()
	r.HandleFunc("/signup", signupHandler).Methods("POST")
	r.HandleFunc("/login", loginHandler).Methods("POST")
	// protected routes
	r.Handle("/notes", authMiddleware(http.HandlerFunc(createNoteHandler))).Methods("POST")
	r.Handle("/notes", authMiddleware(http.HandlerFunc(getNotesHandler))).Methods("GET")

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))

}

//** NOTE-> After user login, server creates a token (jwt)
//** which is basically a signed id saying your id and token expiration time
//** the token is given to client, with each req the client sends token in authorization(header)
//** server will check if token is valid & not expired.
//** server doesn't store session, it just checks token signature.

//**JWT token has 3 parts:
//**1.header-> which algo is used for signing
//**2.payload
//**3.proof that server created this algo (contains a secret key)

/**
OVERALL FLOW-> user signs up for first time , server hashes the password
and store in d.b,in login user sends (username,pass), server matches this against
stored hashed pass, if valid creates a JWT token and sends back to user.
The purpose of middleware is to check if token is valid and not tampered with,
after validation it will pass req through handler.
*/
