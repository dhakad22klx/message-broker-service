# WhatsApp Message Broker & RAG Service

A resilient message processing system built with **Go**, **Kafka (KRaft)**, and **Caddy**. This architecture ensures that WhatsApp chat history is captured and queued even if your downstream processing/RAG server is offline, providing a robust buffer and ensuring zero message loss.

---

## Prerequisites

* **AWS EC2 Instance:** (I used ubuntu m7i-flex.large).
* **Domain Name:** Pointed to your EC2 Elastic IP via GoDaddy (e.g., `broker.dsquare.site`).
* **Meta Developer App:** Set up for WhatsApp Business API.

## Architecture & Flow
The system acts as a resilient asynchronous bridge between Meta's WhatsApp API and your internal AI processing logic.



1.  **Caddy (Reverse Proxy):** Handles automatic SSL termination and routes traffic via subdomains.
2.  **Go Producer:** Validates Meta's `VERIFY_TOKEN`, receives incoming webhooks, and publishes them to Kafka.
3.  **Kafka Cluster (KRaft):** Provides a persistent, ordered message log. It ensures that even during server restarts, messages are safely queued.
4.  **Go Consumer:** Implements a **Partition-Aware Worker Pool** to process messages simultaneously while guaranteeing user-level message ordering.

---

## Infrastructure Requirements

This stack is optimized for performance and requires modern compute resources to handle the Java-based Kafka cluster and concurrent Go routines effectively.

* **Recommended Instance:** `m7i-flex.large` (2 vCPU, 8GB RAM).
* **Minimum Specs:** 2 vCPU, 4GB RAM (Standard t3.small instances may struggle with memory pressure).
* **Operating System:** Ubuntu 22.04+ LTS.
* **Network:** Elastic IP associated with the EC2 instance.

---

## Server Setup & Deployment

### 1. Initial EC2 Preparation
Install the Docker engine and Compose plugin on your Ubuntu instance:

```bash
# Update and install Docker + Compose V2
sudo apt update && sudo apt install -y docker.io docker-compose-v2
sudo usermod -aG docker ubuntu

# Log out and log back in to apply group changes
exit
```

### 2. Environment Configuration

**Deployment** is automated via **GitHub Actions**. You must configure the following **Secrets** in your repository (**Settings** > **Secrets and variables** > **Actions**):

| Secret Name | Description |
| :--- | :--- |
| **HOST** | Your Elastic IP (e.g., `65.1.19.198`). |
| **USER** | SSH Username (default: `ubuntu`). |
| **EC2_SSH_KEY_ED255_A** | The full content of your `.pem` private key. |
| **FASTAPI_URL** | The endpoint where the Consumer forwards messages. |
| **VERIFY_TOKEN** | The unique string used for the Meta Webhook handshake. |



## Reverse Proxy & Security (Caddy)

Caddy manages SSL certificates automatically. The monitoring dashboard is secured using IP Whitelisting to ensure only authorized users can access the Kafka metrics.

```caddy
# Production Webhook Endpoint
broker.dsquare.site {
    reverse_proxy producer:8080
}

# Kafka Visual Monitoring

#kafka monitoring URL
monitor-kafka.dsquare.site {
    reverse_proxy kafka-ui:8080
}

#Option to keep it authorized
monitor-kafka.dsquare.site {
    # Replace with your local machine's public IP
    @allowed_ip remote_ip YOUR_HOME_IP
    
    handle @allowed_ip {
        reverse_proxy kafka-ui:8080
    }

    # Fallback to block unauthorized access
    handle {
        abort
    }
    # Keep this protected with a password!
    basic_auth {
        admin $2a$14$YourHashedPasswordHere
    }
}
```


### Yet To Implement/Futher Goals: Partition-Aware Worker Pool

To maintain the correct order of messages for each user while processing them in parallel, the Consumer utilizes a partition-based worker pattern.

* **Concurrency:** Multiple workers process different Kafka partitions simultaneously.
* **Ordering:** Since messages from the same phone number (the Kafka key) always land in the same partition, a dedicated worker ensures they are processed sequentially.
* **Resilience:** If the processing server is down, messages stay in Kafka. Once back online, the Consumer picks up exactly where it left off.


