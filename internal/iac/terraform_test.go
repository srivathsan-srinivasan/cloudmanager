package iac

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudmanager/internal/core"
)

func TestLoadTerraformStateFileParsesAWSInstance(t *testing.T) {
	path := writeState(t, `{
		"resources": [
			{
				"mode": "managed",
				"type": "aws_instance",
				"name": "web",
				"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
				"instances": [
					{
						"attributes": {
							"id": "i-123",
							"private_ip": "10.0.0.10",
							"public_ip": "203.0.113.10",
							"tags": {"Name": "web-01"}
						}
					}
				]
			}
		]
	}`)

	index, err := LoadTerraformStateFile(path)
	if err != nil {
		t.Fatalf("LoadTerraformStateFile returned error: %v", err)
	}
	if len(index.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(index.Resources))
	}
	resource := index.Resources[0]
	if resource.Address != "aws_instance.web" || resource.Provider != "AWS" || resource.Kind != KindVM {
		t.Fatalf("unexpected resource metadata: %+v", resource)
	}
	if resource.ID != "i-123" || resource.Name != "web-01" || resource.PrivateIP != "10.0.0.10" {
		t.Fatalf("unexpected resource identity: %+v", resource)
	}
}

func TestApplyTerraformToVMsAddsLabelsOnMatch(t *testing.T) {
	path := writeState(t, `{
		"resources": [
			{
				"mode": "managed",
				"type": "aws_instance",
				"name": "web",
				"instances": [
					{"attributes": {"id": "i-123", "name": "web-01"}}
				]
			}
		]
	}`)

	vms := ApplyTerraformToVMs([]string{path}, core.CloudContext{Provider: "AWS"}, []core.VM{
		{ID: "i-123", Name: "web-01", Labels: "env=prod"},
		{ID: "i-456", Name: "other"},
	})

	if !strings.Contains(vms[0].Labels, "iac:terraform") || !strings.Contains(vms[0].Labels, "tf:aws_instance.web") {
		t.Fatalf("expected terraform labels on matched VM, got %q", vms[0].Labels)
	}
	if strings.Contains(vms[1].Labels, "iac:terraform") {
		t.Fatalf("did not expect terraform labels on unmatched VM, got %q", vms[1].Labels)
	}
}

func TestApplyTerraformToDatabasesAndClusters(t *testing.T) {
	path := writeState(t, `{
		"resources": [
			{
				"mode": "managed",
				"type": "aws_db_instance",
				"name": "main",
				"instances": [{"attributes": {"id": "db-main", "identifier": "main-db"}}]
			},
			{
				"mode": "managed",
				"type": "google_container_cluster",
				"name": "primary",
				"instances": [{"attributes": {"id": "projects/p/locations/us/clusters/gke-main", "name": "gke-main"}}]
			}
		]
	}`)

	dbs := ApplyTerraformToDatabases([]string{path}, core.CloudContext{Provider: "AWS"}, []core.Database{
		{ID: "db-main", Name: "main-db"},
	})
	clusters := ApplyTerraformToClusters([]string{path}, core.CloudContext{Provider: "GCP"}, []core.Cluster{
		{Name: "gke-main"},
	})

	if !strings.Contains(dbs[0].Labels, "tf:aws_db_instance.main") {
		t.Fatalf("expected terraform label on database, got %q", dbs[0].Labels)
	}
	if !strings.Contains(clusters[0].Labels, "tf:google_container_cluster.primary") {
		t.Fatalf("expected terraform label on cluster, got %q", clusters[0].Labels)
	}
}

func writeState(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return path
}
