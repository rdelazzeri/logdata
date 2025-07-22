package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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

	if dbPath == "" || port == "" {
		log.Fatal("Missing required environment variables: DATABASE_PATH or PORT")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	http.HandleFunc("/logdata", handlePostLogData(db))
	http.HandleFunc("/getdata", handleGetLogData(db))

	log.Printf("Starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handlePostLogData(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to %s", r.Method, r.URL.Path)
		if r.Method != http.MethodPost {
			log.Printf("Method not allowed: %s", r.Method)
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		account := r.Header.Get("X-Account")
		if account == "" {
			log.Printf("Missing X-Account header")
			http.Error(w, `{"error":"X-Account header required"}`, http.StatusBadRequest)
			return
		}

		var logData LogData
		if err := json.NewDecoder(r.Body).Decode(&logData); err != nil {
			log.Printf("Invalid request body: %v", err)
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		if err := logData.Validate(); err != nil {
			log.Printf("Validation failed: %v", err)
			http.Error(w, fmt.Sprintf(`{"error":"Validation failed: %v"}`, err), http.StatusBadRequest)
			return
		}

		if logData.Account != account {
			log.Printf("Account mismatch: body=%s, header=%s", logData.Account, account)
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
		log.Printf("Received %s request to %s", r.Method, r.URL.Path)
		if r.Method != http.MethodGet {
			log.Printf("Method not allowed: %s", r.Method)
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()
		account := query.Get("account")
		if account == "" {
			log.Printf("Missing account query parameter")
			http.Error(w, `{"error":"Account query parameter required"}`, http.StatusBadRequest)
			return
		}

		params := QueryParams{
			Account:   account,
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
		sqlQuery += " ORDER BY id DESC"
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