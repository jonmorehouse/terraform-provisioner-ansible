package main

import (
	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/terraform"
	"testing"
)

func TestResourceProvisioner_impl(t *testing.T) {
	var _ terraform.ResourceProvisioner = new(ResourceProvisioner)
}

// builds and returns a terraform.ResourceConfig object pointer from a map of generic types
func testConfig(t *testing.T, c map[string]interface{}) *terraform.ResourceConfig {
	r, err := config.NewRawConfig(c)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}

	return terraform.NewResourceConfig(r)
}

func TestResourceProvider_Validate_good(t *testing.T) {
	c := testConfig(t, map[string]interface{}{
		"groups":      []interface{}{"all", "terraform"},
		"module_path": createTmpDir(t, "ansible"),
		"playbook":    createTmpFile(t, "playbook.yml"),
		"plays":       []interface{}{"test", "test"},
		"extra_vars": []interface{}{map[string]string{
			"test": "key",
		}},
	})

	r := new(ResourceProvisioner)
	warn, errs := r.Validate(c)

	if len(warn) > 0 {
		t.Fatalf("Warnings were not expected")
	}

	if len(errs) > 0 {
		t.Fatalf("Errors were not expected")
	}
}
