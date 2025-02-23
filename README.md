# DesVault - Decentralized Encrypted Storage

DesVault is a decentralized storage network that ensures privacy and security by leveraging  cryptographic techniques. It provides a robust, peer-to-peer platform for storing and retrieving files, offering full control over your data in a scalable and resilient ecosystem. Whether you prefer local storage or seamless cloud integration, DesVault empowers users to manage their digital assets with high reliability and performance—ideal for privacy advocates, developers, and tech enthusiasts alike.


## 🌟 Features
- **Peer-to-Peer Encrypted Storage:** Secure, decentralized file storage with robust encryption to protect your data.
- **Hybrid Storage Options:** Seamlessly supports both local storage and cloud integration.
- **Decentralized Node Networking:** Reliable peer discovery with support for multiple seed nodes to ensure robust connectivity.
- **Real-Time Analytics & Health Monitoring:** Get live insights into system performance, network connectivity, and overall node health.
- **Comprehensive API & CLI Tools:** Manage and monitor your storage node effortlessly with secure, rate-limited API endpoints and an intuitive command-line interface.
- **Modular & Scalable Architecture:** Designed for easy expansion and integration of future features.
- **Future Blockchain Integration:** Planned support for integrating blockchain technology for enhanced decentralization and trust.

## 🚀 Run a Storage Node

### Prerequisites
Ensure you have the following installed:
- **Go (1.21+)**
- **Docker** (for running dependencies like IPFS)
- **Git** (to clone the repository)

### Install Go and Docker

#### Linux (Ubuntu/Debian)
```bash
sudo apt update && sudo apt install -y golang-go docker.io git
```

#### Windows

Download and install Go from golang.org.
Install Docker Desktop from docker.com.
Install Git from git-scm.com.

#### macOS
```bash
brew install go docker git
```

### Clone and Run the Node

Clone the repository and start the storage node:
```bash
git clone https://github.com/ArguableExorcist8/desvault-storage-node.git
cd desvault-storage-node
./desvault run
```

### This command will start several services in one terminal:

-**IPFS Daemon (runs on port 5001)**: Handles decentralized file networking.

- **API Server (runs on port 8080)**: Manages file uploads, downloads, and other API requests.

- **Secure QUIC Channel (runs on port 4242)**: Provides TLS-based secure communications.

- **Node Networking**: Manages peer discovery and seed node integration.

### 📊 File Upload/Download Specifications

- **Upload Speed (Moderate)**: 5–10 Mbps
- **Download Speed (Moderate)**: 10–25 Mbps
- **Maximum File Size**: 500 MB per file (files exceeding this limit will be rejected)

## 🔗 Repository  
[DesVault Storage Node](https://github.com/ArguableExorcist8/desvault-storage-node) - A hybrid open-source storage node for decentralized storage.

🛠 Advanced Configuration
### Master API & Seed Nodes
**Master API**:
Coordinates shard distribution, authentication, and reputation tracking. A hybrid failover setup is planned with a secondary API on a different cloud provider for redundancy.

**Seed Nodes (Decentralized Bootstrap)**:
Multiple seed nodes can be run by different users to enhance network discovery and reliability.

**Logging & Monitoring**: 
Ensure proper logging is set up for debugging and performance monitoring.

**Security Best Practices**: 
Regularly update dependencies and monitor for security advisories.

