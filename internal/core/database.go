package core

var DefaultDatabaseColumns = []string{"Name", "Engine", "Version", "Status", "Region", "Size"}

type Database struct {
	Name    string
	ID      string
	Engine  string
	Version string
	Status  string
	Region  string
	Size    string
	Labels  string
}

func (d Database) GetID() string   { return d.ID }
func (d Database) GetName() string { return d.Name }
func (d Database) GetKind() string { return "Database" }
func (d Database) GetField(col string) string {
	switch col {
	case "Name": return d.Name
	case "ID": return d.ID
	case "Engine": return d.Engine
	case "Version": return d.Version
	case "Status": return d.Status
	case "Region": return d.Region
	case "Size": return d.Size
	case "Labels": return d.Labels
	default: return "-"
	}
}
