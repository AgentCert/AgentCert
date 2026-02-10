#!/bin/bash
# Fix minikube Docker DNS

minikube ssh << 'SSHEOF'
sudo tee /etc/docker/daemon.json > /dev/null << 'JSONEOF'
{
  "dns": ["8.8.8.8", "1.1.1.1", "8.8.4.4"],
  "exec-opts":["native.cgroupdriver=systemd"],
  "log-driver":"json-file",
  "log-opts":{"max-size":"100m"},
  "storage-driver":"overlay2"
}
JSONEOF

echo "✓ DNS config updated"
sudo systemctl restart docker
echo "✓ Docker restarted"
SSHEOF
