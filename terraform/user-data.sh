#!/bin/bash
# Bootstrap script for the Monitor Secure Pipeline EC2 instance.
# Installs Docker + Compose and starts the observability stack.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

echo "==> Updating system packages"
apt-get update -qq
apt-get install -y -qq \
  ca-certificates curl gnupg lsb-release git jq unattended-upgrades

echo "==> Installing Docker"
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

echo "==> Enabling Docker"
systemctl enable docker
systemctl start docker
usermod -aG docker ubuntu

echo "==> Enabling unattended security upgrades"
cat > /etc/apt/apt.conf.d/50unattended-upgrades <<'EOF'
Unattended-Upgrade::Allowed-Origins {
  "${distro_id}:${distro_codename}-security";
};
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "03:00";
EOF

echo "==> Cloning repository"
if [ -d /opt/monitor-secure-pipeline ]; then
  cd /opt/monitor-secure-pipeline
  git pull --ff-only origin main || true
else
  git clone --depth 1 ${repo_url} /opt/monitor-secure-pipeline
  cd /opt/monitor-secure-pipeline
fi

echo "==> Creating .env from template"
if [ ! -f .env ]; then
  cp .env.example .env
fi

echo "==> Starting the stack"
docker compose pull --quiet || true
docker compose up -d --build

echo "==> Stack is up. Summary:"
docker compose ps --format 'table {{.Name}}\t{{.Status}}\t{{.Ports}}' || true

echo "==> Grafana will be available at http://${public_ip}:3000 (admin/admin)"
