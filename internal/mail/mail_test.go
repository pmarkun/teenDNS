package mail

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendLogOnlyWithoutAPIKey(t *testing.T) {
	client := NewResendClient("", "gateway@teendns.test")
	client.Endpoint = "http://unused.invalid"

	if err := client.Send("family@example.com", "Assunto", "<p>Oi</p>"); err != nil {
		t.Fatalf("Send in log-only mode returned error: %v", err)
	}
}

func TestSendPostsToResendAPI(t *testing.T) {
	var receivedAuth string
	var receivedBody resendRequest

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAuth = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewResendClient("test-key", "gateway@teendns.test")
	client.Endpoint = server.URL

	if err := client.Send("family@example.com", "Assunto", "<p>Oi</p>"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if receivedAuth != "Bearer test-key" {
		t.Fatalf("unexpected Authorization header %q", receivedAuth)
	}
	if receivedBody.From != "gateway@teendns.test" {
		t.Fatalf("unexpected From %q", receivedBody.From)
	}
	if len(receivedBody.To) != 1 || receivedBody.To[0] != "family@example.com" {
		t.Fatalf("unexpected To %v", receivedBody.To)
	}
	if receivedBody.Subject != "Assunto" {
		t.Fatalf("unexpected Subject %q", receivedBody.Subject)
	}
}

func TestSendReturnsErrorOnNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"invalid api key"}`))
	}))
	defer server.Close()

	client := NewResendClient("bad-key", "gateway@teendns.test")
	client.Endpoint = server.URL

	err := client.Send("family@example.com", "Assunto", "<p>Oi</p>")
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected error to mention status code, got %v", err)
	}
}
