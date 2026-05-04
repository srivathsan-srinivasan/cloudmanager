package azure

import "testing"

func TestAzureContextNameSanitizesSubscriptionName(t *testing.T) {
	got := azureContextName("Azure Sponsorship - Engineering", "sub-1")
	if got != "azure-sponsorship-engineering" {
		t.Fatalf("expected sanitized azure context name, got %q", got)
	}
}

func TestAzureContextNameFallsBackToSubscriptionID(t *testing.T) {
	got := azureContextName(" ", "sub-fallback")
	if got != "sub-fallback" {
		t.Fatalf("expected subscription id fallback, got %q", got)
	}
}
