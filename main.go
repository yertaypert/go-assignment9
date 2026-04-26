package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/yertaypert/go-assignment9/pkg/payment"
)

func main() {
	var counter int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&counter, 1)

		if current <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "temporary failure")
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
		})
	}))
	defer server.Close()

	client := &http.Client{}

	req, _ := http.NewRequest("POST", server.URL, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := payment.ExecutePayment(ctx, client, req, 5)
	if err != nil {
		log.Fatalf("Final error: %v", err)
	}
	defer resp.Body.Close()

	log.Println("Final response received successfully")
}
