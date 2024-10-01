
# Distributed Orchestration and Consensus

A project designed to implement and test distributed orchestration and consensus algorithms for fault-tolerant, scalable systems.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgments](#acknowledgments)

## Overview

This repository contains implementations of distributed orchestration and consensus algorithms. The consensus algorithms ensure fault-tolerant agreements between distributed nodes. These are key in distributed systems where coordination across multiple services, databases, or network nodes is required.

### Key objectives:
- **Distributed orchestration**: Efficient distributed orchestration system to handle microservices deployment and migration.
- **Consensus Protocols**: Implementation of modified Raft consensus protocol tied for orchestration.
- **Fault tolerance**: Algorithms ensure agreement even in case of node failures.

## Features

- Implementation of modified Raft distributed consensus algorithm.
- Fault-tolerant coordination across distributed nodes.
- Distributed and consistent log system through the use of GlusterFS distributed filesystem.
- Scalable design for testing across multiple nodes.
- Support for custom orchestration setups.
  
## Prerequisites

Before running or developing this project, ensure you have the following installed:

- **Docker** (For running containerized instances, mandatory)
- **Distributed Systems** knowledge (Recommended)

## Installation

Clone the repository on each node and navigate into the project directory:

```bash
git clone https://github.com/fcrlab-unime/distributed-orchestration-consensus.git
cd distributed-orchestration-consensus
```

### Build docker images:

1. Ensure Docker is installed:
   ```bash
   docker --version
   ```

2. Make build script executable:
   ```bash
   sudo chmod +x build.sh
   ```

3. Execute the build script:
   ```bash
   ./build.sh
   ```
## Usage

To run the system, use the following command on each node:

```bash
docker compose up -d
```

This will launch the distributed filesystem and the orchestration and consensus algorithm based on the specified configuration.

**N.B.**: the system must be launched on the other nodes only after it is ready on the first one.

## Configuration

All configurations for the orchestration setup and consensus algorithms are managed through .env file. This file contains parameters like:

- **RPC_PORT**: port for internal communication
- **GATEWAY_PORT**: port for incoming orchestration requests
- **LOG_PATH**: path where the distributed file system is mounted
- **DEBUG**: [0,1] enable/disable debug features
- **NET_IFACE**: network interface of the host node

## Contributing

We welcome contributions from the community! Please follow these steps:

1. Fork the repository.
2. Create a new branch (`git checkout -b feature-branch`).
3. Make your changes.
4. Submit a pull request detailing your changes.

Make sure to include relevant tests and update documentation when submitting pull requests.

## License

This project is licensed under the GNU General Public License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

This project is maintained by the [FCRLab-Unime](https://github.com/fcrlab-unime) team. We appreciate contributions and collaboration with the distributed systems research community.
