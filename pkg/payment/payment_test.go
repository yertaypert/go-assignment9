package payment

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestExecutePayment_Retries(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		if atomic.LoadInt32(&attempts) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL, nil)
	ctx := context.Background()

	resp, err := ExecutePayment(ctx, client, req, 5)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestExecutePayment_MaxAttempts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL, nil)
	ctx := context.Background()

	_, err := ExecutePayment(ctx, client, req, 3)
	if err == nil {
		t.Fatal("expected error after max attempts, got nil")
	}
}

func TestExecutePayment_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel immediately
	cancel()

	_, err := ExecutePayment(ctx, client, req, 5)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
