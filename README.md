# terraform-provisioner-ansible
An attempt at provisioning terraform instances with ansible

Still a WIP but this `terraform` plugin provides a basic provisioner for
bootstrapping `terraform` instance resources with ansible. 

As a first pass, this uses a `local-ansible.py` script for running ansible
locally on the host. Ansible code is shipped to the resource and run locally.
This is to avoid having to introduce ansible as a dependency at the `terraform`
layer and to simplify the responsibilities of this plugin.



