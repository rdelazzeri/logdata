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
		secretKey := strings.TrimPrefix(authHeader, "Bearer ")

		var account string
		if r.Method == http.MethodPost {
			account = r.Header.Get("X-Account")
			if account == "" {
				http.Error(w, `{"error":"X-Account header required"}`, http.StatusBadRequest)
				return
			}
		} else if r.Method == http.MethodGet {
			account = r.URL.Query().Get("account")
			if account == "" {
				http.Error(w, `{"error":"Account query parameter required"}`, http.StatusBadRequest)
				return
			}
		}

		if expectedKey, ok := accountSecretKeys[account]; !ok || secretKey != expectedKey {
			http.Error(w, `{"error":"Unauthorized: Invalid secret key for account"}`, http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

func handlePostLogData(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var logData LogData
		if err := json.NewDecoder(r.Body).Decode(&logData); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		if err := logData.Validate(); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Validation failed: %v"}`, err), http.StatusBadRequest)
			return
		}

		account := r.Header.Get("X-Account")
		if logData.Account != account {
			http.Error(w, `{"error":"Account in body must match X-Account header"}`, http.StatusBadRequest)
			return
		}

		_, err := db.Exec(
			`INSERT INTO logData (account, system, user, module, task, timestamp, msg, level)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			logData.Account, logData.System, logData.User, logData.Module,
			logData.Task, logData.Timestamp, logData.Msg, logData.Level,
		)
		if err != nil {
			log.Printf("Error saving log data: %v", err)
			http.Error(w, `{"error":"Failed to save log data"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Log data saved successfully"})
	}
}

func handleGetLogData(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()
		params := QueryParams{
			Account:   query.Get("account"),
			System:    query.Get("system"),
			User:      query.Get("user"),
			Module:    query.Get("module"),
			Task:      query.Get("task"),
			Level:     nil,
			StartTime: query.Get("start_time"),
			EndTime:   query.Get("end_time"),
			Limit:     nil,
			Offset:    nil,
		}

		var level int
		if query.Get("level") != "" {
			if _, err := fmt.Sscanf(query.Get("level"), "%d", &level); err == nil {
				params.Level = &level
			}
		}

		var limit, offset int64 = 100, 0
		if query.Get("limit") != "" {
			if _, err := fmt.Sscanf(query.Get("limit"), "%d", &limit); err == nil {
				params.Limit = &limit
			}
		}
		if query.Get("offset") != "" {
			if _, err := fmt.Sscanf(query.Get("offset"), "%d", &offset); err == nil {
				params.Offset = &offset
			}
		}

		sqlQuery := "SELECT id, account, system, user, module, task, timestamp, msg, level FROM logData WHERE account = ?"
		args := []interface{}{params.Account}
		if params.System != "" {
			sqlQuery += " AND system = ?"
			args = append(args, params.System)
		}
		if params.User != "" {
			sqlQuery += " AND user = ?"
			args = append(args, params.User)
		}
		if params.Module != "" {
			sqlQuery += " AND module = ?"
			args = append(args, params.Module)
		}
		if params.Task != "" {
			sqlQuery += " AND task = ?"
			args = append(args, params.Task)
		}
		if params.Level != nil {
			sqlQuery += " AND level = ?"
			args = append(args, *params.Level)
		}
		if params.StartTime != "" {
			sqlQuery += " AND timestamp >= ?"
			args = append(args, params.StartTime)
		}
		if params.EndTime != "" {
			sqlQuery += " AND timestamp <= ?"
			args = append(args, params.EndTime)
		}
		sqlQuery += " ORDER BY timestamp DESC"
		if params.Limit != nil {
			sqlQuery += fmt.Sprintf(" LIMIT %d", *params.Limit)
		}
		if params.Offset != nil {
			sqlQuery += fmt.Sprintf(" OFFSET %d", *params.Offset)
		}

		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			log.Printf("Error querying log data: %v", err)
			http.Error(w, `{"error":"Failed to fetch log data"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var logs []LogData
		for rows.Next() {
			var logData LogData
			var id int64
			if err := rows.Scan(&id, &logData.Account, &logData.System, &logData.User,
				&logData.Module, &logData.Task, &logData.Timestamp, &logData.Msg, &logData.Level); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			logData.ID = &id
			logs = append(logs, logData)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
	}
}