package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
)

//const AnsibleLocal = "https://github.com/jonmorehouse/terraform-provi

// this implements the tfrpc.ProvisionerFunc type and returns an instance of a
// resourceProvisioner when called
func ResourceProvisionerBuilder() terraform.ResourceProvisioner {

	return &ResourceProvisioner{}
}

func main() {
	serveOpts := &plugin.ServeOpts{
		ProvisionerFunc: ResourceProvisionerBuilder,
	}

	plugin.Serve(serveOpts)
}
