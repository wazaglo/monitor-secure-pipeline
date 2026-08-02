terraform {
  required_version = ">= 1.3"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

data "aws_ssm_parameter" "ubuntu_ami" {
  name = "/aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp2/ami-id"
}

resource "aws_security_group" "observability_sg" {
  name        = "${var.project_name}-sg"
  description = "Monitor Secure Pipeline security group"

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "SSH"
  }

  ingress {
    from_port   = 3000
    to_port     = 3000
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Grafana"
  }

  ingress {
    from_port   = 9090
    to_port     = 9090
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Prometheus"
  }

  ingress {
    from_port   = 3100
    to_port     = 3100
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Loki"
  }

  ingress {
    from_port   = 3200
    to_port     = 3200
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Tempo"
  }

  ingress {
    from_port   = 9093
    to_port     = 9093
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Alertmanager"
  }

  ingress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "API Gateway"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-sg"
    Project = var.project_name
  }
}

resource "aws_key_pair" "observability" {
  key_name   = "${var.project_name}-key"
  public_key = var.public_key
  tags = {
    Name    = "${var.project_name}-key"
    Project = var.project_name
  }
}

resource "aws_eip" "observability" {
  domain = "vpc"
  tags = {
    Name    = "${var.project_name}-eip"
    Project = var.project_name
  }
}

resource "aws_instance" "observability" {
  ami                    = data.aws_ssm_parameter.ubuntu_ami.value
  instance_type          = var.instance_type
  key_name               = aws_key_pair.observability.key_name
  vpc_security_group_ids = [aws_security_group.observability_sg.id]

  root_block_device {
    volume_type = "gp3"
    volume_size = var.root_volume_size
    tags = {
      Name    = "${var.project_name}-root"
      Project = var.project_name
    }
  }

  user_data = templatefile("${path.module}/user-data.sh", {
    repo_url  = var.repo_url
    public_ip = aws_eip.observability.public_ip
  })

  user_data_replace_on_change = true

  tags = {
    Name    = "${var.project_name}-instance"
    Project = var.project_name
  }
}

resource "aws_eip_association" "observability" {
  instance_id   = aws_instance.observability.id
  allocation_id = aws_eip.observability.id
}
