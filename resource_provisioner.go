package main

import (
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/mapstructure"
	"log"
	"time"
)

type ResourceProvisioner struct {
	playbook  string // filepath for the
	inventory string

	groups    []string          // list of group-vars to be passed to the provisioner
	hosts     []string          // list of host groups to apply to the provisioner
	extraVars map[string]string // extra variables that can be passed to the provisioner
}

func (r *ResourceProvisioner) Apply(
	o terraform.UIOutput,
	s *terraform.InstanceState,
	c *terraform.ResourceConfig) error {

	provisioner, err := r.decodeConfig(c)
	if err != nil {
		o.Output("erred out here")
		return err
	}

	err = provisioner.Validate()
	if err != nil {
		o.Output("Invalid provisioner configuration settings")
		return err
	}
	provisioner.useSudo = true
	provisioner.ansibleLocalScript = fmt.Sprintf("https://raw.githubusercontent.com/joaocc/jonmorehouse--terraform-provisioner-ansible/%s/ansible-local.py", VERSION)

	// ensure that this is a linux machine
	if s.Ephemeral.ConnInfo["type"] != "ssh" {
		return fmt.Errorf("Unsupported connection type: %s. This provisioner currently only supports linux", s.Ephemeral.ConnInfo["type"])
	}

	// build a communicator for the provisioner to use
	comm, err := communicator.New(s)
	if err != nil {
		o.Output("erred out here 3")
		return err
	}

	err = retryFunc(comm.Timeout(), func() error {
		err := comm.Connect(o)
		return err
	})
	if err != nil {
		return err
	}
	defer comm.Disconnect()

	err = provisioner.Run(o, comm)
	if err != nil {
		o.Output("erred out here 4")
		return err
	}

	return nil
}

func (r *ResourceProvisioner) Validate(c *terraform.ResourceConfig) (ws []string, es []error) {
	provisioner, err := r.decodeConfig(c)
	if err != nil {
		es = append(es, err)
		return ws, es
	}

	err = provisioner.Validate()
	if err != nil {
		es = append(es, err)
		return ws, es
	}

	return ws, es
}

func (r *ResourceProvisioner) decodeConfig(c *terraform.ResourceConfig) (*Provisioner, error) {
	// decodes configuration from terraform and builds out a provisioner
	p := new(Provisioner)
	decoderConfig := &mapstructure.DecoderConfig{
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		Result:           p,
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, err
	}

	// build a map of all configuration values, by default this is going to
	// pass in all configuration elements for the base configuration as
	// well as extra values. Build a single value and then from there, continue forth!
	m := make(map[string]interface{})
	for k, v := range c.Raw {
		m[k] = v
	}
	for k, v := range c.Config {
		m[k] = v
	}

	err = decoder.Decode(m)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func retryFunc(timeout time.Duration, f func() error) error {
	finish := time.After(timeout)

	for {
		err := f()
		if err == nil {
			return nil
		}
		log.Printf("Retryable error: %v", err)

		select {
		case <-finish:
			return err
		case <-time.After(3 * time.Second):
		}
	}
}
