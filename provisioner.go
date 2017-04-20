package main

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/communicator/remote"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-homedir"
	"github.com/mitchellh/go-linereader"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Provisioner struct {
	useSudo            bool
	ansibleLocalScript string
	Playbook           string            `mapstructure:"playbook"`
	Plays              []string          `mapstructure:"plays"`
	Hosts              []string          `mapstructure:"hosts"`
	ModulePath         string            `mapstructure:"module_path"`
	Groups             []string          `mapstructure:"groups"` // group_vars are expected to be under <ModulePath>/group_var/name
	ExtraVars          map[string]string `mapstructure:"extra_vars"`
}

func (p *Provisioner) Run(o terraform.UIOutput, comm communicator.Communicator) error {
	// parse the playbook path and ensure that it is valid before doing
	// anything else. This is done in validate but is repeated here, just
	// in case.
	playbookPath, err := p.resolvePath(p.Playbook)
	if err != nil {
		return err
	}

	// commands that are needed to setup a basic environment to run the `ansible-local.py` script
	// TODO pivot based upon different platforms and allow optional python provision steps
	// TODO this should be configurable for folks who want to customize this
	provisionAnsibleCommands := []string{
		// https://github.com/hashicorp/terraform/issues/1025
		// cloud-init runs on fresh sources and can interfere with apt-get update commands causing intermittent failures
		"/bin/bash -c 'until [[ -f /var/lib/cloud/instance/boot-finished ]]; do sleep 1; done'",
		"apt-get update",
		"apt-get install -y build-essential python-dev",
		"curl https://bootstrap.pypa.io/get-pip.py | sudo python",
		"pip install ansible",
	}

	for _, command := range provisionAnsibleCommands {
		o.Output(fmt.Sprintf("running command: %s", command))
		err := p.runCommand(o, comm, command)
		if err != nil {
			return err
		}
	}

	// ansible projects are structured such that the playbook file is in
	// the top level of the module path. As such, we parse the playbook
	// path's directory and upload the entire thing
	playbookDir := filepath.Dir(playbookPath)

	// the host playbook path is the path on the host where the playbook
	// will be uploaded too
	remotePlaybookPath := filepath.Join("/tmp/ansible", filepath.Base(playbookPath))

	// upload ansible source and playbook to the host
	if err := comm.UploadDir("/tmp/ansible", playbookDir); err != nil {
		return err
	}

	extraVars, err := json.Marshal(p.ExtraVars)
	if err != nil {
		return err
	}

	// build a command to run ansible on the host machine
	command := fmt.Sprintf("curl %s | python - --playbook=%s --hosts=%s --plays=%s --groups=%s --extra-vars=%s",
		p.ansibleLocalScript,
		remotePlaybookPath,
		strings.Join(p.Hosts, ","),
		strings.Join(p.Plays, ","),
		strings.Join(p.Groups, ","),
		string(extraVars))

	o.Output(fmt.Sprintf("running command: %s", command))
	if err := p.runCommand(o, comm, command); err != nil {
		return err
	}

	return nil
}

func (p *Provisioner) Validate() error {
	playbookPath, err := p.resolvePath(p.Playbook)
	if err != nil {
		return err
	}
	p.Playbook = playbookPath

	for _, host := range p.Hosts {
		if host == "" {
			return fmt.Errorf("Invalid hosts parameter. hosts: %s", p.Hosts)
		}
	}

	for _, play := range p.Plays {
		if play == "" {
			return fmt.Errorf("Invalid plays paramter. plays: %s", p.Plays)
		}
	}

	for _, group := range p.Groups {
		if group == "" {
			return fmt.Errorf("Invalid group. groups: %s", p.Groups)
		}
	}

	for _, host := range p.Hosts {
		if host == "" {
			return fmt.Errorf("Invalid host. hosts: %s", p.Hosts)
		}
	}

	return nil
}

func (p *Provisioner) runCommand(
	o terraform.UIOutput,
	comm communicator.Communicator,
	command string) error {

	var err error
	if p.useSudo {
		command = "sudo " + command
	}

	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	outDoneCh := make(chan struct{})
	errDoneCh := make(chan struct{})

	go p.copyOutput(o, outR, outDoneCh)
	go p.copyOutput(o, errR, errDoneCh)

	cmd := &remote.Cmd{
		Command: command,
		Stdout:  outW,
		Stderr:  errW,
	}

	if err := comm.Start(cmd); err != nil {
		return fmt.Errorf("Error executing command %q: %v", cmd.Command, err)
	}
	cmd.Wait()
	if cmd.ExitStatus != 0 {
		err = fmt.Errorf(
			"Command %q exited with non-zero exit status: %d", cmd.Command, cmd.ExitStatus)

	}

	outW.Close()
	errW.Close()
	<-outDoneCh
	<-errDoneCh

	return err
}

func (p *Provisioner) copyOutput(o terraform.UIOutput, r io.Reader, doneCh chan<- struct{}) {
	defer close(doneCh)
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}

func (p *Provisioner) resolvePath(path string) (string, error) {
	expandedPath, _ := homedir.Expand(path)
	if _, err := os.Stat(expandedPath); err == nil {
		return expandedPath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Unable to get current working address to resolve path as a relative path")
	}

	relativePath := filepath.Join(cwd, path)
	if _, err := os.Stat(relativePath); err == nil {
		return relativePath, nil
	}

	return "", fmt.Errorf("Path not valid: [%s]", relativePath)
}

