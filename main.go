package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/yertaypert/go-assignment9/pkg/idempotency"
)

func main() {
	// 1. Setup Idempotency Middleware
	store := idempotency.NewMemoryStore()
	idemMiddleware := idempotency.Middleware(store)

	// 2. Setup Handler with specific "Processing started" log
	paymentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("[Server] Processing started...")
		
		// Simulate heavy operation
		time.Sleep(2 * time.Second)

		resp := map[string]interface{}{
			"status":         "paid",
			"amount":         1000,
			"transaction_id": "uuid-9876-5432-1098",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		
		log.Println("[Server] Processing completed.")
	})

	server := httptest.NewServer(idemMiddleware(paymentHandler))
	defer server.Close()

	client := &http.Client{}
	idemKey := "double-click-key-test"
	
	const numConcurrent = 10
	var wg sync.WaitGroup
	wg.Add(numConcurrent)

	log.Printf("--- Simulating 'Double-Click' Attack with %d concurrent requests ---", numConcurrent)

	// 3. Launch concurrent requests
	for i := 1; i <= numConcurrent; i++ {
		go func(id int) {
			defer wg.Done()
			
			req, _ := http.NewRequest("POST", server.URL, nil)
			req.Header.Set("Idempotency-Key", idemKey)
			
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Request %d error: %v", id, err)
				return
			}
			defer resp.Body.Close()
			
			log.Printf("Request %d finished. Status: %d", id, resp.StatusCode)
		}(i)
	}

	wg.Wait()

	// 4. Verify cached result after completion
	log.Println("--- Verifying cached result after first completion ---")
	req, _ := http.NewRequest("POST", server.URL, nil)
	req.Header.Set("Idempotency-Key", idemKey)
	
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Verification request error: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	log.Printf("Final request status: %d, Result: %v", resp.StatusCode, result)
	log.Println("Note: Server log should NOT show 'Processing started' for this final request.")
}
