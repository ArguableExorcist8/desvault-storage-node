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

## 🛠 Advanced Configuration
### Master API & Seed Nodes
**Master API**:
Coordinates shard distribution, authentication, and reputation tracking. A hybrid failover setup is planned with a secondary API on a different cloud provider for redundancy.

**Seed Nodes (Decentralized Bootstrap)**:
Multiple seed nodes can be run by different users to enhance network discovery and reliability.

**Logging & Monitoring**: 
Ensure proper logging is set up for debugging and performance monitoring.

**Security Best Practices**: 
Regularly update dependencies and monitor for security advisories.

### Feel free to fork, star, or contribute to the project. For any issues or feature requests, please open an issue or submit a pull request.

## 📝 License
This project is licensed under the Apache-2.0 license. See the LICENSE file for details.

## 📄 Changelog
#### Version pre-release 0.1:
Initial release featuring decentralized encrypted storage, file management, and Real-Time Analytics.

## 🔮 Future Enhancements

- **Blockchain Integration:**
Planned support for integrating blockchain technology to further enhance decentralization, data integrity, and trust.
-**Enhanced API Failover Mechanisms:**
Implementation of  failover strategies and redundant API endpoints to ensure uninterrupted service, even in the event of primary endpoint issues.
-**Performance Optimizations:**
Ongoing improvements to boost file processing speeds, reduce latency, and scale the network efficiently as data volumes and user demand grow.
-**Collaborative Chat & Real-Time Collaboration:**
Advanced real-time chat features for enhanced communication between node operators and users, including integrated tools for collaborative file editing (document, spreadsheet, and presentation editing), in-line commenting, and version control.
-**Integrated Collaborative Document Editors:**
Built‑in document, spreadsheet, and presentation editors that allow multiple users to edit, annotate, and track changes simultaneously, creating a seamless work environment.
-**Marketplace & Plugin Ecosystem:**
Development of a decentralized marketplace to buy, sell, and trade storage capacity and related services. In parallel, launch a plugin ecosystem with robust APIs and SDKs so developers can create and integrate third‑party tools—ranging from creative applications to additional productivity features.
-**Creators Hub:**
Introduction of a dedicated Creators Hub featuring digital asset management, monetization tools (including smart contract-based licensing and rights management), and tools for organizing and sharing creative content.
-**Enhanced File & Media Editing Tools:**
Beyond basic document editing, include native image, audio, and video editing capabilities to empower creators to make quick edits without leaving the platform.
-**Mobile and Desktop Applications:**
Native apps for both mobile and desktop platforms to ensure full editing, collaboration, and file management capabilities are available on the go.
-**Advanced Version Control and File History:**
Implement comprehensive version tracking, allowing users to view, compare, and restore previous versions of their files seamlessly.