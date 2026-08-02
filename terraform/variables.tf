variable "region" {
  description = "AWS region"
  default     = "us-east-1"
}

variable "instance_type" {
  description = "EC2 instance type"
  default     = "t3.medium"
}

variable "root_volume_size" {
  description = "Root EBS volume size in GB"
  default     = 30
}

variable "public_key" {
  description = "SSH public key content for EC2 access"
  type        = string
}

variable "project_name" {
  description = "Project name for resource tagging"
  default     = "monitor-secure-pipeline"
}

variable "repo_url" {
  description = "URL of the GitHub repo"
  type        = string
}
