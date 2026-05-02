package core

// AccessMethod describes one way to reach a VM or machine.
type AccessMethod struct {
	ID        string
	Kind      string
	Label     string
	Priority  int
	Command   []string
	CopyText  string
	Available bool
	Reason    string
}
