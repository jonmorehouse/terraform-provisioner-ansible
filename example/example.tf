provider "aws" {
	region = "us-east-1"
}

resource "aws_instance" "ansible-test" {
	ami = "ami-408c7f28"
	instance_type = "t1.micro"

	provisioner "ansible" {
		connection {
			user = "ubuntu"
		}

		playbook = "playbook.yml"
		plays = ["terraform"]
		hosts = ["all"]
		groups = ["terraform"]
	}
}

