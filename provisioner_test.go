package main

import (
	"io/ioutil"
	"testing"
)

func createTmpDir(t *testing.T, directoryPrefix string) string {
	d, err := ioutil.TempDir("", directoryPrefix)
	if err != nil {
		t.Fatalf("Unable to create temporary directory")
	}

	return d
}

func createTmpFile(t *testing.T, filenamePrefix string) string {
	// by default, we want to use the operating system's choice of temporary directory
	f, err := ioutil.TempFile("", filenamePrefix)
	if err != nil {
		t.Fatalf("Unable to create temporary file")
	}

	return f.Name()
}

func TestProvisioner_good(t *testing.T) {
	p := &Provisioner{
		Playbook:   createTmpFile(t, "playbook"),
		Plays:      []string{"play1", "play2"},
		ModulePath: createTmpDir(t, "ansible"),
		Groups:     []string{"all", "terraform"},
		ExtraVars: map[string]string{
			"key": "value",
		},
	}

	if err := p.Validate(); err != nil {
		t.Log(err)
		t.Fatalf("unable to validate properly configured provisioner")
	}
}
