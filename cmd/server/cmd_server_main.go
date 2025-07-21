package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// LogData represents a log entry in the logData table.
type LogData struct {
	ID        *int64    `json:"id,omitempty"`
	Account   string    `json:"account"`
	System    string    `json:"system"`
	User      string    `json:"user"`
	Module    string    `json:"module"`
	Task      string    `json:"task"`
	Timestamp time.Time `json:"timestamp"`
	Msg       string    `json:"msg"`
	Level     int       `json:"level"`
}

// Validate ensures LogData has required fields.
func (l LogData) Validate() error {
	if l.Account == "" || l.System == "" || l.User == "" || l.Module == "" || l.Task == "" || l.Msg == "" {
		return fmt.Errorf("missing required fields")
	}
	if l.Timestamp.IsZero() {
		return fmt.Errorf("invalid timestamp")
	}
	return nil
}

// QueryParams represents query parameters for GET /getdata.
type QueryParams struct {
	Account   string `json:"account"`
	System    string `json:"system"`
	User      string `json:"user"`
	Module    string `json:"module"`
	Task      string `json:"task"`
	Level     *int   `json:"level"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Limit     *int64 `json:"limit"`
	Offset    *int64 `json:"offset"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dbPath := os.Getenv("DATABASE_PATH")
	port := os.Getenv("PORT")
	accountSecretKeysJSON := os.Getenv("ACCOUNT_SECRET_KEYS")

	if dbPath == "" || port == "" || accountSecretKeysJSON == "" {
		log.Fatal("Missing required environment variables: DATABASE_PATH, PORT, or ACCOUNT_SECRET_KEYS")
	}

	var accountSecretKeys map[string]string
	if err := json.Unmarshal([]byte(accountSecretKeysJSON), &accountSecretKeys); err != nil {
		log.Fatalf("Failed to parse ACCOUNT_SECRET_KEYS: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	http.HandleFunc("/logdata/", authMiddleware(accountSecretKeys, handlePostLogData(db)))
	http.HandleFunc("/getdata", authMiddleware(accountSecretKeys, handleGetLogData(db)))

	log.Printf("Starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// authMiddleware checks if the provided secret key matches the account in the request.
func authMiddleware(accountSecretKeys map[string]string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"Invalid or missing Authorization header"}`, http.StatusUnauthorized)
			return
		}
		secretKey := strings.TrimPrefix(authHeader, "Bearer