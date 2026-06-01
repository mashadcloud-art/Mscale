package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ACLRule struct {
	ID        string    `json:"id"`
	SourceIP  string    `json:"source_ip"`
	DestIP    string    `json:"dest_ip"`
	Port      int       `json:"port"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

func HandleACLs(db *sql.DB, auth *AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := auth.GetSession(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet {
			rows, err := db.Query("SELECT id, source_ip, dest_ip, port, action, created_at FROM acls WHERE user_id = ? ORDER BY created_at DESC", session.UserID)
			if err != nil {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var rules []ACLRule
			for rows.Next() {
				var rule ACLRule
				if err := rows.Scan(&rule.ID, &rule.SourceIP, &rule.DestIP, &rule.Port, &rule.Action, &rule.CreatedAt); err != nil {
					continue
				}
				rules = append(rules, rule)
			}
			if rules == nil {
				rules = []ACLRule{}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rules)
			return
		}

		if r.Method == http.MethodPost {
			var rule ACLRule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			
			rule.ID = uuid.New().String()
			if rule.Action == "" {
				rule.Action = "DROP"
			}
			rule.Action = strings.ToUpper(rule.Action)

			_, err := db.Exec(
				"INSERT INTO acls (id, user_id, source_ip, dest_ip, port, action) VALUES (?, ?, ?, ?, ?, ?)",
				rule.ID, session.UserID, rule.SourceIP, rule.DestIP, rule.Port, rule.Action,
			)
			if err != nil {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(rule)
			return
		}
		
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func HandleACLDelete(db *sql.DB, auth *AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		
		session, err := auth.GetSession(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		id := parts[3]

		_, err = db.Exec("DELETE FROM acls WHERE id = ? AND user_id = ?", id, session.UserID)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}
}
