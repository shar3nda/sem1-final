#!/bin/bash
set -e
set -o pipefail

YC_FOLDER_ID=$YC_FOLDER_ID
YC_ZONE=${YC_ZONE:-ru-central1-a}
YC_IMAGE_ID="fd8bnguet48kpk4ovt1u"
INSTANCE_NAME="prices-app-$(date +%s)"
SSH_USER="ubuntu"
SSH_KEY_PATH="$HOME/.ssh/id_rsa"
INSTANCE_TYPE="standard-v1"

echo ">>> Creating VM instance..."
YC_INSTANCE_ID=$(yc compute instance create \
	--name "$INSTANCE_NAME" \
	--folder-id "$YC_FOLDER_ID" \
	--zone "$YC_ZONE" \
	--network-interface subnet-name=default,nat-ip-version=ipv4 \
	--create-boot-disk size=20,image-id="$YC_IMAGE_ID" \
	--memory=2 \
	--cores=2 \
	--ssh-key "${SSH_KEY_PATH}.pub" \
	--format json | jq -r '.id')

echo "Instance ID: $YC_INSTANCE_ID"

PUBLIC_IP=$(yc compute instance get --id "$YC_INSTANCE_ID" --format json | jq -r '.network_interfaces[0].primary_v4_address.one_to_one_nat.address')

echo "Public IP: $PUBLIC_IP"

echo "Trying to SSH..."
for i in {1..20}; do
	if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$SSH_USER@$PUBLIC_IP" "echo ok" >/dev/null 2>&1; then
		break
	fi
	echo "waiting... $i/20"
	sleep 10
done

echo "Installing Docker and Compose..."
ssh "$SSH_USER@$PUBLIC_IP" <<'EOF'
sudo apt remove -y $(dpkg --get-selections docker.io docker-compose docker-compose-v2 docker-doc podman-docker containerd runc | cut -f1)
sudo apt update -y
sudo apt install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

sudo bash -c 'cat <<EOT > /etc/apt/sources.list.d/docker.sources
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: '$(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")'
Components: stable
Signed-By: /etc/apt/keyrings/docker.asc
EOT'

sudo apt update

sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable docker --now
sudo usermod -aG docker $USER
EOF

echo "Copying deployment files..."
scp docker-compose.yml "$SSH_USER@$PUBLIC_IP:/home/$SSH_USER/"
scp .env "$SSH_USER@$PUBLIC_IP:/home/$SSH_USER/"

echo "Starting Docker Compose..."
ssh "$SSH_USER@$PUBLIC_IP" <<'EOF'
docker compose pull >/dev/null 2>&1
docker compose up -d
EOF

echo "PUBLIC_IP=$PUBLIC_IP" >>$GITHUB_ENV
echo "API_HOST=http://$PUBLIC_IP:8080" >>$GITHUB_ENV
echo "DB_HOST=$PUBLIC_IP" >>$GITHUB_ENV
