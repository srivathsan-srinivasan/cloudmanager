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
- `disks`
- `snapshots`
- `networks`
- `firewalls`
- `ssh`
- `cost_guide`

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
- optional `disks.go`
- optional `snapshots.go`
- optional `billing.go`
- tests

## Required Data Shape

Provider code should map resources into the common core models:

- `core.CloudContext`
- `core.VM`
- `core.Disk`
- `core.Snapshot`

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

