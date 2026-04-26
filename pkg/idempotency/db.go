package idempotency

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	query := `
	CREATE TABLE IF NOT EXISTS idempotency_keys (
		key TEXT PRIMARY KEY,
		status TEXT,
		status_code INTEGER,
		response_body BYTEA
	);`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	return &SQLStore{db: db}
}

func (s *SQLStore) Get(key string) (*Record, bool) {
	var record Record
	var body []byte

	query := `SELECT status, status_code, response_body FROM idempotency_keys WHERE key = $1`
	err := s.db.QueryRow(query, key).Scan(&record.Status, &record.StatusCode, &body)

	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		return nil, false
	}

	record.ResponseBody = body
	return &record, true
}

func (s *SQLStore) SetIfAbsent(key string, record Record) bool {
	query := `INSERT INTO idempotency_keys (key, status, status_code, response_body) 
			  VALUES ($1, $2, $3, $4) 
			  ON CONFLICT (key) DO NOTHING`

	res, err := s.db.Exec(query, key, record.Status, record.StatusCode, record.ResponseBody)
	if err != nil {
		return false
	}

	rows, _ := res.RowsAffected()
	return rows > 0
}

func (s *SQLStore) Update(key string, record Record) {
	query := `UPDATE idempotency_keys SET status = $1, status_code = $2, response_body = $3 WHERE key = $4`
	_, err := s.db.Exec(query, record.Status, record.StatusCode, record.ResponseBody, key)
	if err != nil {
		log.Printf("Failed to update database record: %v", err)
	}
}
