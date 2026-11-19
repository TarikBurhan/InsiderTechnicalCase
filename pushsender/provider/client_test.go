package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"notifier/pushsender/request"
)

func TestSendAccepted(t *testing.T) {
	var got request.ProviderRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != "delivery-1-1" {
			t.Errorf("missing idempotency key")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"messageId":"m-1","status":"accepted","timestamp":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	resp, err := NewClient(srv.URL, time.Second).Send(context.Background(),
		request.ProviderRequest{To: "device-token-abc123", Channel: "push", Content: "hi"}, "delivery-1-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.MessageID != "m-1" || resp.Status != "accepted" {
		t.Fatalf("unexpected response %+v", resp)
	}
	if got.To != "device-token-abc123" || got.Channel != "push" || got.Content != "hi" {
		t.Fatalf("unexpected request %+v", got)
	}
}

func TestSendErrorClassification(t *testing.T) {
	cases := map[int]bool{ // status -> permanent
		http.StatusInternalServerError: false,
		http.StatusServiceUnavailable:  false,
		http.StatusTooManyRequests:     false,
		http.StatusBadRequest:          true,
		http.StatusUnprocessableEntity: true,
	}
	for status, permanent := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		_, err := NewClient(srv.URL, time.Second).Send(context.Background(), request.ProviderRequest{}, "k")
		srv.Close()
		if err == nil {
			t.Fatalf("status %d: expected error", status)
		}
		if IsPermanent(err) != permanent {
			t.Fatalf("status %d: permanent=%v, want %v", status, IsPermanent(err), permanent)
		}
	}
}

func TestSendNetworkErrorIsRetryable(t *testing.T) {
	_, err := NewClient("http://127.0.0.1:1", 200*time.Millisecond).Send(context.Background(), request.ProviderRequest{}, "k")
	if err == nil || IsPermanent(err) {
		t.Fatalf("network errors must be retryable, got %v", err)
	}
}
