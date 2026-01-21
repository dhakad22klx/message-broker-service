# Update and install Docker
sudo apt update && sudo apt install -y docker.io docker-compose-v2
sudo usermod -aG docker ubuntu
# Log out and log back in for group changes to take effect
exit



Github Actions Secrets worked well with ed25519 encrypted key pair.

Inside caddy, Authentication Option : Skipped for now. 

monitor-kafka.dsquare.site {
    # Replace 'YOUR_HOME_IP' with your actual public IP from whatismyip.com
    @allowed_ip remote_ip YOUR_HOME_IP
    
    handle @allowed_ip {
        reverse_proxy kafka-ui:8080
    }

    # Block everyone else
    handle {
        abort
    }
}


# Keep this protected with a password!
basic_auth {
    admin $2a$14$YourHashedPasswordHere
}