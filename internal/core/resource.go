package core

// Resource is the generic interface for any cloud resource.
// Views render Resources into tables; providers return Resources from APIs.
type Resource interface {
	GetID() string
	GetName() string
	GetKind() string       // e.g. "VM", "Network", "Disk"
	GetField(col string) string
}

// Action represents a user-triggerable action on a resource.
type Action struct {
	Title       string
	Description string
	Dangerous   bool // requires confirmation dialog
}
