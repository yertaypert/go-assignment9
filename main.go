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
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/yertaypert/go-assignment9/pkg/idempotency"
	"github.com/yertaypert/go-assignment9/pkg/payment"
)

func main() {
	// Load env
	if err := godotenv.Load(); err != nil {
		log.Println("[Info] No .env file found, using system environment variables")
	}

	fmt.Println("=========================================================")
	fmt.Println("TASK 1: Unstable Server Simulation (Retry Logic)")
	fmt.Println("=========================================================")
	runTask1()

	fmt.Println("\n=========================================================")
	fmt.Println("TASK 2: Idempotency Protection (Double-Click Attack)")
	fmt.Println("=========================================================")
	runTask2()
}

// runTask1 demonstrates: 3 failures (503), then success (200) with Backoff & Jitter
func runTask1() {
	var counter int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&counter, 1)

		if current <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Service Unavailable")
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("POST", server.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_, err := payment.ExecutePayment(ctx, client, req, 5)
	if err != nil {
		log.Printf("Final error in Task 1: %v", err)
	}
}

// runTask2 demonstrates: Concurrent requests, 409 Conflict, and Postgres work
func runTask2() {
	// Setup DB
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"), os.Getenv("DB_SSLMODE"),
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	var store idempotency.Store
	if err := db.Ping(); err != nil {
		log.Println("[Warning] DB not reachable, using MemoryStore for Demo Task 2")
		store = idempotency.NewMemoryStore()
	} else {
		store = idempotency.NewSQLStore(db)
	}

	idemMiddleware := idempotency.Middleware(store)

	// Logic handler
	paymentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("[Server] Processing started (Business Logic)...")
		time.Sleep(2 * time.Second)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "paid",
			"amount":         1000,
			"transaction_id": "uuid-" + time.Now().Format("05"),
		})
		log.Println("[Server] Processing completed.")
	})

	server := httptest.NewServer(idemMiddleware(paymentHandler))
	defer server.Close()

	client := &http.Client{}
	idemKey := "demo-key-" + time.Now().Format("150405")

	const numConcurrent = 10
	var wg sync.WaitGroup
	wg.Add(numConcurrent)

	log.Printf("Sending %d concurrent requests with key: %s\n", numConcurrent, idemKey)

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
			log.Printf("Request %d -> Status: %d", id, resp.StatusCode)
		}(i)
	}

	wg.Wait()

	log.Println("\n--- Final Check (Should be served from DB cache) ---")
	req, _ := http.NewRequest("POST", server.URL, nil)
	req.Header.Set("Idempotency-Key", idemKey)
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	log.Printf("Verification Request -> Status: %d", resp.StatusCode)
}
