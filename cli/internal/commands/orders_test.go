package commands

import "testing"

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
