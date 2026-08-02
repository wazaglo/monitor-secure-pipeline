# Deploy to AWS

The same stack runs on EC2 using Terraform. It provisions a t3.medium instance
(30 GB gp3 root volume) with Docker, clones the repo, and starts the platform.

## Prerequisites

- AWS credentials configured (`aws configure`)
- Terraform 1.3+
- An SSH keypair to access the instance

## 1. Prepare variables

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
repo_url   = "https://github.com/wazaglo/monitor-secure-pipeline.git"
public_key = "ssh-rsa AAAA..."   # your public key
```

Optionally set `instance_type` (default `t3.medium`) and
`root_volume_size` (default `30`).

## 2. Apply

```bash
terraform init
terraform plan
terraform apply
```

## 3. What you get

| Output | Value |
| ------ | ----- |
| `grafana_url` | http://PUBLIC_IP:3000 (admin/admin) |
| `prometheus_url` | http://PUBLIC_IP:9090 |
| `api_gateway_url` | http://PUBLIC_IP:8080 |
| `setup_instructions` | Full next-step summary |

## 4. Connect the DefectDojo exporter

The exporter is disabled from scraping until you point it at a real DefectDojo:

```bash
ssh -i <key.pem> ubuntu@PUBLIC_IP
cd /opt/monitor-secure-pipeline
export DD_API_URL=http://<defectdojo-ip>:8080
export DD_API_TOKEN=<token>
docker compose up -d --build defectdojo-exporter
```

The exporter then publishes `defectdojo_findings{severity,...}` to Prometheus,
and the Security dashboard + HighSeverityFindings alerts become active.

## 5. Tear down

```bash
terraform destroy
```

## Security notes

- In production, restrict the security-group CIDRs in `terraform/main.tf` to
  your office/VPN instead of `0.0.0.0/0`.
- Grafana default creds are `admin/admin` — change them in
  `docker-compose.yml` env vars or wire in Grafana provisioning auth.
- Unattended-upgrades are enabled by the user-data script for patch cadence.
