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
)

type Provisioner struct {
	useSudo            bool
	ansibleLocalScript string
	Playbook           string            `mapstructure:"playbook"`
	Plays              []string          `mapstructure:"plays"`
	ModulePath         string            `mapstructure:"module_path"`
	Groups             []string          `mapstructure:"groups"` // group_vars are expected to be under <ModulePath>/group_var/name
	ExtraVars          map[string]string `mapstructure:"extra_vars"`
}

func (p *Provisioner) Run(o terraform.UIOutput, comm communicator.Communicator) error {
	// commands that are needed to setup a basic environment to run the `ansible-local.py` script
	// TODO pivot based upon different platforms and allow optional python provision steps
	provisionAnsibleCommands := []string{
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

	// upload ansible source and playbook to the host
	if err := comm.UploadDir("/tmp/ansible-module-path", p.ModulePath); err != nil {
		return err
	}

	playbook, err := os.Open(p.Playbook)
	if err != nil {
		return err
	}
	defer playbook.Close()
	if err := comm.Upload("/tmp/ansible-playbook", playbook); err != nil {
		return err
	}

	extraVars, err := json.Marshal(p.ExtraVars)
	if err != nil {
		return err
	}

	// build run command!
	command := fmt.Sprintf("curl %s | python --module-path=%s --playbook=%s --plays=%s --groups=%s --extra-vars=%s",
		p.ansibleLocalScript,
		p.ModulePath,
		p.Playbook,
		strings.Join(",", p.Plays),
		strings.Join(",", p.Groups),
		string(extraVars))

	if err := p.runCommand(o, comm, command); err != nil {
		return err
	}

	return nil
}

func (p *Provisioner) Validate() error {
	return nil

	modulePath, err := homedir.Expand(p.ModulePath)
	if err != nil {
		return fmt.Errorf("ModulePath is not a valid filepath: %s", p.ModulePath)
	}

	stat, err := os.Stat(modulePath)
	if err != nil && os.IsNotExist(err) || stat.IsDir() {
		return fmt.Errorf("ModulePath is not a valid directory. path: %s", p.ModulePath)
	}

	playbookPath, err := homedir.Expand(p.Playbook)
	if err != nil {
		return fmt.Errorf("Playbook is not a valid filepath. path: %s", p.Playbook)
	}

	_, err = os.Stat(playbookPath)
	if err != nil && os.IsNotExist(err) {
		return fmt.Errorf("Playbook is not a valid filepath. path: %s", p.Playbook)
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
