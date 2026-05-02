# Provider Authoring Guide

CloudManager is meant to be a provider-neutral TUI for primary infrastructure operations.

The UI contract is intentionally small:

1. select provider context
2. browse compute resources
3. inspect resources
4. run simple actions
5. optionally open guidance such as SSH, cost lookup, or console links

## Design Rules

- Do not add provider-specific UI branches unless they are strictly unavoidable.
- Normalize provider quirks inside the provider package before data reaches the UI.
- Treat unsupported capabilities as normal. A provider may support only VMs.
- Prefer simple, operational workflows over deep billing or reporting logic.
- Keep advanced guidance on-demand.

## Registry Model

Providers are registered in `internal/providers/registry.go`.

Each provider contributes:

- metadata
- context discovery
- capability map
- CLI bindings
- SDK bindings

The current capability set is:

- `vms`
- `hosts`
- `disks`
- `snapshots`
- `networks`
- `firewalls`
- `clusters`
- `databases`
- `storage`
- `ssh`
- `cost_guide`
- `metrics`

## Provider Tiers

CloudManager should stay lightweight. Do not make every new cloud SDK a mandatory dependency.

Use this order:

1. Manual host support
   - Use this when the provider is not worth an API integration yet.
   - Store IP/DNS, username, SSH config alias, key refs, and CloudManager tags in config.
   - No provider SDK or CLI dependency.

2. CLI provider
   - Use this when the provider has a stable CLI that can output JSON.
   - Keep parsing inside `internal/providers/<provider>/`.
   - This is acceptable for first support because auth stays with the vendor CLI.

3. SDK provider
   - Use this when the provider has a useful Go SDK and the integration is mature enough.
   - Prefer SDK code behind optional build tags or a provider sidecar if the SDK is large.
   - Do not import large long-tail SDKs into the default binary without a clear reason.

4. External provider process
   - Future direction for community providers.
   - A provider binary can speak a small JSON protocol to CloudManager.
   - This keeps the main `cloudmanager` binary portable while letting providers move independently.

## Where A New Provider Goes

For a provider such as OVHcloud:

1. Add provider code under:

   `internal/providers/ovhcloud/`

2. Add context discovery:

   `internal/providers/ovhcloud/parser.go`

3. Add VM operations:

   `internal/providers/ovhcloud/client.go`

4. Register provider metadata and bindings in:

   `internal/providers/registry.go`

5. Add regression tests:

   `internal/providers/ovhcloud/client_test.go`
   `internal/providers/registry_test.go`

If the provider only supports VMs initially, register only:

- `CapabilityVMs`
- `CapabilitySSH`
- optionally `CapabilityCostGuide`

Unsupported tabs must remain hidden.

## Minimal Provider Contract

The smallest useful provider can implement only:

- context discovery
- VM listing
- simple VM actions
- SSH command or SSH guidance

That is enough for the provider to appear in the sidebar and work in the VM tab.

## Recommended File Layout

Create a new folder:

`internal/providers/<provider-name>/`

Typical files:

- `parser.go`
- `client.go`
- optional `auth.go`
- optional `disks.go`
- optional `snapshots.go`
- optional `firewalls.go`
- optional `networks.go`
- optional `clusters.go`
- optional `databases.go`
- optional `storage.go`
- optional `metrics.go`
- optional `billing.go`
- tests

## Required Data Shape

Provider code should map resources into the common core models:

- `core.CloudContext`
- `core.VM`
- `core.Disk`
- `core.Snapshot`
- `core.StorageBucket`

The UI should only consume normalized models, not raw provider payloads.

## Registration Steps

1. Add the provider package.
2. Register it in `internal/providers/registry.go`.
3. Define metadata:
   - provider id
   - display name
   - ordering
   - aliases
   - capabilities
   - optional global leaf label
4. Add `LoadContexts`.
5. Bind CLI and/or SDK implementations for each supported capability.
6. Add tests.

Minimal registration shape:

```go
RegisterProvider(RegisteredProvider{
	Metadata: ProviderMetadata{
		ID:          "OVHcloud",
		DisplayName: "OVHcloud",
		Order:       100,
		Aliases:     []string{"ovh", "ovhcloud"},
		Capabilities: map[Capability]bool{
			CapabilityVMs: true,
			CapabilitySSH: true,
		},
	},
	LoadContexts: ovhcloud.LoadContexts,
	CLI: backendBindings{
		Compute: computeFuncs{
			fetch: ovhcloud.FetchVMsCLI,
			execute: ovhcloud.ExecuteVMActionCLI,
			ssh: ovhcloud.GetSSHCmd,
		},
	},
})
```

If OVHcloud ships a large SDK, do not pull it into the default binary blindly.
Prefer an optional SDK build or an external provider process.

## Sidebar Behavior

The sidebar is built from discovered or managed `CloudContext` values.

Providers should emit contexts in the shape:

- `Provider`
- `AccountID`
- `AccountName`
- `Region`
- optional `CredentialProfile`

Use `Region = "global"` when the provider should show a single `All Resources` leaf instead of regional children.

## Tab Behavior

Tabs are capability-driven.

If a provider supports only VMs, only the VM tab should be shown.
Do not fake support for disks or snapshots just to keep the tab count uniform.

## Cost Behavior

Cost is optional.

Preferred order:

1. `CostGuide`
2. estimated cost
3. actual billing cost

Do not block provider support on exact billing integrations.

## Proof Standard

A new provider is considered integrated when:

- it can appear in the context tree
- the VM tab works
- simple actions do not break the shell
- unsupported tabs stay hidden
- global search can index its VMs
- CloudManager-only tags work on its VMs

## Lightweight Binary Rule

The default `cloudmanager` binary should remain useful on a fresh machine.

Default build should include:

- core TUI
- config and local index support
- manual hosts
- common built-in providers already accepted by the project

Default build should avoid:

- every niche cloud SDK
- large transitive dependencies for optional providers
- provider-specific UI branches

Long-tail SDK providers should graduate through one of these paths:

- CLI-only first
- optional build tag
- external provider process

## SDK Update Policy

SDK upgrades are allowed, but they must be scoped and reversible.

CloudManager currently pins SDK dependencies in `go.mod`. Do not use floating
versions or broad dependency rewrites just to get one provider fix.

Use this order:

1. Upgrade only the SDK modules needed for the provider or bug.
2. Prefer patch or minor upgrades.
3. Avoid major upgrades unless the provider API requires it.
4. Do not mix SDK upgrades with unrelated UI or provider refactors.
5. Run `go mod tidy` only after checking the `go.mod` diff is reasonable.

Provider-specific examples:

- AWS SDK v2 modules are split per service. Upgrade the specific service module first, such as `service/ec2`, before broad AWS SDK upgrades.
- Azure SDK packages are split by ARM service and sometimes major path, such as `armnetwork/v6`. Do not collapse or replace versions casually.
- GCP uses both `cloud.google.com/go/...` clients and `google.golang.org/api/...` clients. Keep auth fallback behavior intact when upgrading.

Required validation for SDK upgrades:

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/<provider>`
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

Also verify the affected provider behavior:

- context discovery still works
- VM listing still maps into `core.VM`
- action commands still use context/profile/subscription/project correctly
- SDK auth still works or fails with a useful message
- GCP SDK paths keep CLI-auth fallback where already implemented
- global search indexing still receives normalized resources
- unsupported capabilities still stay hidden

When an SDK introduces API-shape changes:

- adapt inside `internal/providers/<provider>/`
- keep `core.*` models stable unless the product behavior truly needs a model change
- add regression tests for the mapping or auth behavior that changed

When an SDK increases binary size noticeably:

- justify it in the PR
- consider build tags or an external provider process
- do not add large long-tail SDKs to the default binary without review

Upgrade PR notes should include:

- old and new SDK versions
- provider/resource areas affected
- auth mode tested
- commands/tests run
- any live-cloud validation skipped

## Global Access And Configure Surface

CloudManager should have one global entry point for:

- resources already configured
- credentials that need setup
- manual hosts
- provider login
- local tags
- index refresh

Current surfaces:

- `g`: global resource search
- `,`: settings
- `:`: command entry
- `:login`: provider login
- `:add-provider`: managed provider context
- `:add-host`: manual host

Target UX:

- one command palette key
- show configured resources first
- show missing provider setup actions second
- show local actions such as tags, index refresh, settings, and logs last

This should be implemented on top of the existing command and settings actions, not as a separate workflow.
