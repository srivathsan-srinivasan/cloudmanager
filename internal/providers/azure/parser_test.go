package azure

import "testing"

func TestAzureContextNameSanitizesSubscriptionName(t *testing.T) {
	got := azureContextName("Azure Sponsorship - Engineering", "sub-1")
	if got != "azure-sponsorship-engineering" {
		t.Fatalf("expected sanitized azure context name, got %q", got)
	}
}

func TestAzureContextNameFallsBackToSubscriptionID(t *testing.T) {
	got := azureContextName(" ", "83ea0471")
	if got != "83ea0471" {
		t.Fatalf("expected subscription id fallback, got %q", got)
	}
}
