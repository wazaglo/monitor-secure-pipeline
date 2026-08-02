output "public_ip" {
  description = "EC2 public IP address"
  value       = aws_eip.observability.public_ip
}

output "instance_id" {
  description = "EC2 instance ID"
  value       = aws_instance.observability.id
}

output "grafana_url" {
  description = "Grafana dashboard URL (admin/admin)"
  value       = "http://${aws_eip.observability.public_ip}:3000"
}

output "prometheus_url" {
  description = "Prometheus URL"
  value       = "http://${aws_eip.observability.public_ip}:9090"
}

output "api_gateway_url" {
  description = "API Gateway URL"
  value       = "http://${aws_eip.observability.public_ip}:8080"
}

output "setup_instructions" {
  description = "Next steps after provisioning"
  value = <<-EOF
    ─────────────────────────────────────────────
    EC2 is ready with the Monitor Secure Pipeline stack.

    Grafana:     http://${aws_eip.observability.public_ip}:3000   (admin / admin)
    Prometheus:  http://${aws_eip.observability.public_ip}:9090
    Loki:        http://${aws_eip.observability.public_ip}:3100
    Tempo:       http://${aws_eip.observability.public_ip}:3200
    API Gateway: http://${aws_eip.observability.public_ip}:8080

    SSH: ssh -i <your-key-file.pem> ubuntu@${aws_eip.observability.public_ip}
    Services: docker compose ls
    Logs:     docker compose logs -f --tail=100

    ── 1. Point DefectDojo exporter at your DefectDojo ──
    export DD_API_URL=http://<defectdojo-ip>:8080
    export DD_API_TOKEN=<token>
    docker compose up -d defectdojo-exporter

    ── 2. (Optional) Restrict security group CIDRs in production ──
  EOF
}
