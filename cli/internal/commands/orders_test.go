package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrderRequestEndpoint(t *testing.T) {
	got, err := orderRequestEndpoint("https://seller.example.com/base/")
	if err != nil {
		t.Fatal(err)
	}

	want := "https://seller.example.com/base/agent-nexus/request"
	if got != want {
		t.Fatalf("endpoint mismatch: want %s got %s", want, got)
	}
}

func TestOrderRequestEndpointRequiresAbsoluteURL(t *testing.T) {
	_, err := orderRequestEndpoint("seller.example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSubmitOrderRequest(t *testing.T) {
	var got orderRequestPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method mismatch: %s", r.Method)
		}
		if r.URL.Path != "/agent-nexus/request" {
			t.Fatalf("path mismatch: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"seller_confirmed","orderId":"7"}`))
	}))
	defer server.Close()

	response, err := submitOrderRequest(t.Context(), server.URL+"/agent-nexus/request", orderRequestPayload{
		MarketAddress: "0x1111111111111111111111111111111111111111",
		OrderID:       "7",
		Request:       "hello",
		Signature:     "0xsig",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderID != "7" || got.Request != "hello" || got.Signature != "0xsig" {
		t.Fatalf("payload mismatch: %+v", got)
	}
	if response["status"] != "seller_confirmed" {
		t.Fatalf("response mismatch: %+v", response)
	}
}

func TestDeliveryEndpoint(t *testing.T) {
	got, err := deliveryEndpoint("https://seller.example.com/base/")
	if err != nil {
		t.Fatal(err)
	}

	want := "https://seller.example.com/base/agent-nexus/delivery"
	if got != want {
		t.Fatalf("endpoint mismatch: want %s got %s", want, got)
	}
}

func TestDeliveryEndpointRequiresAbsoluteURL(t *testing.T) {
	_, err := deliveryEndpoint("seller.example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDisputeEndpoint(t *testing.T) {
	got, err := disputeEndpoint("https://validator.example.com/base/")
	if err != nil {
		t.Fatal(err)
	}

	want := "https://validator.example.com/base/agent-nexus/disputes"
	if got != want {
		t.Fatalf("endpoint mismatch: want %s got %s", want, got)
	}
}
