# Update and install Docker
sudo apt update && sudo apt install -y docker.io docker-compose
sudo usermod -aG docker ubuntu
# Log out and log back in for group changes to take effect
exit



Github Actions Secrets worked well with ed25519 encrypted key pair.