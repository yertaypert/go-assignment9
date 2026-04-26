package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/yertaypert/go-assignment9/pkg/idempotency"
)

func main() {
	// 1. Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("[Info] No .env file found, using system environment variables")
	}

	// 2. Build Connection String
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	// 3. Setup PostgreSQL Connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check if Postgres is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var store idempotency.Store
	if err := db.PingContext(ctx); err != nil {
		log.Printf("[Warning] PostgreSQL not reachable at %s:%s (%v). Falling back to MemoryStore.",
			os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), err)
		store = idempotency.NewMemoryStore()
	} else {
		log.Printf("[Info] Connected to PostgreSQL database: %s", os.Getenv("DB_NAME"))
		store = idempotency.NewSQLStore(db)
	}

	idemMiddleware := idempotency.Middleware(store)

	// 4. Setup Handler
	paymentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("[Server] Processing started (Business Logic)...")
		time.Sleep(2 * time.Second)

		resp := map[string]interface{}{
			"status":         "paid",
			"amount":         1000,
			"transaction_id": "uuid-postgres-env-demo",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		log.Println("[Server] Processing completed.")
	})

	server := httptest.NewServer(idemMiddleware(paymentHandler))
	defer server.Close()

	// 5. Launch concurrent requests
	client := &http.Client{}
	idemKey := "env-pg-key-" + time.Now().Format("150405")

	const numConcurrent = 10
	var wg sync.WaitGroup
	wg.Add(numConcurrent)

	log.Printf("--- Running PostgreSQL with %d requests ---", numConcurrent)

	for i := 1; i <= numConcurrent; i++ {
		go func(id int) {
			defer wg.Done()
			req, _ := http.NewRequest("POST", server.URL, nil)
			req.Header.Set("Idempotency-Key", idemKey)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			log.Printf("Request %d finished. Status: %d", id, resp.StatusCode)
		}(i)
	}

	wg.Wait()
}
