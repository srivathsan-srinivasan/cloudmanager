package gcp

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
)

func TestLiveDescribeConfiguredGCPContexts(t *testing.T) {
	if os.Getenv("CLOUDMANAGER_LIVE_GCP_DESCRIBE") != "1" {
		t.Skip("set CLOUDMANAGER_LIVE_GCP_DESCRIBE=1 to test live GCP VM describe across configured contexts")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	contexts := configuredGCPContexts()
	if len(contexts) == 0 {
		t.Fatal("no configured GCP contexts found")
	}

	for _, cloudCtx := range contexts {
		cloudCtx := cloudCtx
		t.Run(cloudCtx.AccountID, func(t *testing.T) {
			vms, err := FetchVMsCLI(cloudCtx.AccountID)
			if err != nil {
				t.Fatalf("list VMs failed: %v", err)
			}
			if len(vms) == 0 {
				t.Skip("no VMs in context")
			}

			vm := vms[0]
			cliOutput, err := ExecuteActionCLI(ctx, "Describe", vm, cloudCtx)
			if err != nil {
				t.Fatalf("CLI describe failed for %s/%s zone=%q: %v", cloudCtx.AccountID, vm.Name, vm.Zone, err)
			}
			assertDescribeOutput(t, "cli", cliOutput, vm)
			t.Logf("CLI describe ok project=%s vm=%s zone=%s bytes=%d", cloudCtx.AccountID, vm.Name, vm.Zone, len(cliOutput))

			sdkOutput, err := ExecuteActionSDK(ctx, "Describe", vm, cloudCtx)
			if err != nil {
				t.Fatalf("SDK describe failed for %s/%s zone=%q: %v", cloudCtx.AccountID, vm.Name, vm.Zone, err)
			}
			assertDescribeOutput(t, "sdk", sdkOutput, vm)
			t.Logf("SDK describe ok project=%s vm=%s zone=%s bytes=%d", cloudCtx.AccountID, vm.Name, vm.Zone, len(sdkOutput))
		})
	}
}

func configuredGCPContexts() []core.CloudContext {
	cfg := config.Load()
	seen := map[string]bool{}
	var contexts []core.CloudContext
	for _, cloudCtx := range config.ManagedContexts(cfg) {
		if !strings.EqualFold(cloudCtx.Provider, "GCP") {
			continue
		}
		accountID := strings.TrimSpace(cloudCtx.AccountID)
		if accountID == "" || seen[accountID] {
			continue
		}
		seen[accountID] = true
		contexts = append(contexts, cloudCtx)
	}
	if len(contexts) > 0 {
		return contexts
	}
	discovered, _ := LoadContexts()
	for _, cloudCtx := range discovered {
		accountID := strings.TrimSpace(cloudCtx.AccountID)
		if accountID == "" || seen[accountID] {
			continue
		}
		seen[accountID] = true
		contexts = append(contexts, cloudCtx)
	}
	return contexts
}

func assertDescribeOutput(t *testing.T, mode, output string, vm core.VM) {
	t.Helper()
	output = strings.TrimSpace(output)
	if output == "" {
		t.Fatalf("%s describe returned empty output", mode)
	}
	if !strings.Contains(output, vm.Name) && !strings.Contains(output, vm.ID) {
		t.Fatalf("%s describe output does not mention vm name/id; vm=%s id=%s output_prefix=%q", mode, vm.Name, vm.ID, prefix(output, 300))
	}
}

func prefix(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
