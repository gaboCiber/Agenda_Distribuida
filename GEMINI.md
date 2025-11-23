# Project Overview

This project is a distributed calendar system developed as part of a Distributed Systems course. It is built using a microservices architecture with services written in Go. The system is designed to manage users, groups, and events in a distributed manner.

The main components of the system are:

- **User Service**: Handles user management, authentication, profiles, and credentials.
- **Group Service**: Manages user groups, memberships, permissions, and group hierarchies.
- **DB Service**: A centralized database service for data persistence and transaction management.
- **Redis**: Used for asynchronous communication between services and as a distributed cache.

## Building and Running

### Prerequisites

- Docker 20.10+
- Go 1.19+ (for local development)

### Build

The project uses Docker to build and run the services. The `scripts/build.sh` script builds the Docker images for the services.

**Build all services:**

```bash
./scripts/build.sh all
```

**Build a specific service:**

```bash
./scripts/build.sh [db|user|group]
```

### Running

The `scripts/start.sh` script starts the services.

**Start all services:**

```bash
./scripts/start.sh all
```

**Start a specific service:**

```bash
./scripts/start.sh [redis|db|user|group]
```

### Service Ports

- **User Service**: `http://localhost:8001`
- **Group Service**: `http://localhost:8003`
- **DB Service**: `http://localhost:8005`
- **Redis**: `redis://localhost:6379`

### Stopping

The `scripts/stop.sh` script stops the services.

**Stop all services:**

```bash
./scripts/stop.sh all
```

**Stop a specific service:**

```bash
./scripts/stop.sh [redis|db|user|group]
```

### Development Conventions

The project follows standard Go practices. Each service is structured as a separate module with its own `go.mod` file. The services are designed to be containerized and deployed using Docker.

The code is organized into `cmd` for the main application entry points and `internal` for the core application logic. This is a common pattern in Go projects to enforce package boundaries.
