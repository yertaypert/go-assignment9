package idempotency

import (
	"bytes"
	"net/http"
	"sync"
)

type Status string

const (
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
)

type Record struct {
	Status       Status
	StatusCode   int
	ResponseBody []byte
}

// Store defines the interface for idempotency storage (Redis, DB, etc.)
type Store interface {
	Get(key string) (*Record, bool)
	SetIfAbsent(key string, record Record) bool
	Update(key string, record Record)
}

// MemoryStore is a simple in-memory implementation of Store
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]Record),
	}
}

func (s *MemoryStore) Get(key string) (*Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[key]
	return &r, ok
}

func (s *MemoryStore) SetIfAbsent(key string, record Record) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[key]; ok {
		return false
	}
	s.records[key] = record
	return true
}

func (s *MemoryStore) Update(key string, record Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[key] = record
}

// Middleware creates an idempotency middleware
func Middleware(store Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				http.Error(w, "Idempotency-Key header is missing", http.StatusBadRequest)
				return
			}

			// 1. Check if key exists
			if record, ok := store.Get(key); ok {
				if record.Status == StatusProcessing {
					http.Error(w, "Request already in progress", http.StatusConflict)
					return
				}
				if record.Status == StatusCompleted {
					w.WriteHeader(record.StatusCode)
					w.Write(record.ResponseBody)
					return
				}
			}

			// 2. Try to set as processing
			if !store.SetIfAbsent(key, Record{Status: StatusProcessing}) {
				// Re-check in case it finished or another thread won
				if record, ok := store.Get(key); ok {
					if record.Status == StatusProcessing {
						http.Error(w, "Request already in progress", http.StatusConflict)
						return
					}
					w.WriteHeader(record.StatusCode)
					w.Write(record.ResponseBody)
					return
				}
			}

			// 3. Capture the response
			rec := &responseRecorder{
				ResponseWriter: w,
				body:           new(bytes.Buffer),
			}

			next.ServeHTTP(rec, r)

			// 4. Save the result
			store.Update(key, Record{
				Status:       StatusCompleted,
				StatusCode:   rec.statusCode,
				ResponseBody: rec.body.Bytes(),
			})
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rec *responseRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	if rec.statusCode == 0 {
		rec.statusCode = http.StatusOK
	}
	rec.body.Write(b)
	return rec.ResponseWriter.Write(b)
}
