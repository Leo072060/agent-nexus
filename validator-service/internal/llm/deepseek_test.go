package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientRequiresAPIKey(t *testing.T) {
	_, err := NewClient("", "https://api.deepseek.com", "deepseek-chat")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecideParsesDecisionJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path mismatch: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization mismatch: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "{\"releaseToSeller\":true,\"summary\":\"seller wins\",\"reasoning\":\"delivery matches request\",\"buyerClaim\":\"bad delivery\",\"sellerDeliveryAssessment\":\"sufficient\",\"confidence\":\"high\"}"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL, "deepseek-chat")
	if err != nil {
		t.Fatal(err)
	}

	decision, reportBody, err := client.Decide(context.Background(), Evidence{OrderID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.ReleaseToSeller {
		t.Fatal("expected releaseToSeller=true")
	}
	if len(reportBody) == 0 {
		t.Fatal("expected report body")
	}
}

func TestDecideRejectsInvalidDecisionJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "not json"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL, "deepseek-chat")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = client.Decide(context.Background(), Evidence{OrderID: "12"})
	if err == nil {
		t.Fatal("expected error")
	}
}
