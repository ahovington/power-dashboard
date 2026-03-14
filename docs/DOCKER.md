# Docker Setup

## Overview

This project uses Docker Compose to orchestrate multiple services for local development and testing.

## Services

### 1. PostgreSQL Database

```yaml
db:
  image: postgres:14-alpine
  environment:
    POSTGRES_DB: power_monitor
    POSTGRES_USER: postgres
    POSTGRES_PASSWORD: password
  ports:
    - "5432:5432"
  volumes:
    - postgres_data:/var/lib/postgresql/data
    - ./backend/migrations:/docker-entrypoint-initdb.d
```

### 2. Go Backend API

```yaml
backend:
  build:
    context: .
    dockerfile: docker/Dockerfile.backend
  environment:
    DATABASE_URL: postgres://postgres:password@db:5432/power_monitor
    GO_ENV: development
  ports:
    - "8080:8080"
  depends_on:
    db:
      condition: service_healthy
  volumes:
    - ./backend:/app/backend
```

### 3. React Frontend

```yaml
frontend:
  build:
    context: .
    dockerfile: docker/Dockerfile.frontend
  environment:
    REACT_APP_API_URL: http://localhost:8080
  ports:
    - "3000:3000"
  volumes:
    - ./frontend:/app/frontend
    - /app/frontend/node_modules
  depends_on:
    - backend
```

## Docker Compose Configuration

```yaml
// filepath: /Users/alastairhovington/Documents/Github/power-dashboard/docker-compose.yml
version: '3.8'

services:
  db:
    image: postgres:14-alpine
    container_name: power_monitor_db
    environment:
      POSTGRES_DB: power_monitor
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
      POSTGRES_INITDB_ARGS: "--encoding=UTF8"
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./backend/migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - power_network

  backend:
    build:
      context: .
      dockerfile: docker/Dockerfile.backend
    container_name: power_monitor_backend
    environment:
      DATABASE_URL: postgres://postgres:password@db:5432/power_monitor
      GO_ENV: development
      LOG_LEVEL: debug
      ENPHASE_API_KEY: ${ENPHASE_API_KEY}
      ENPHASE_SYSTEM_ID: ${ENPHASE_SYSTEM_ID}
    ports:
      - "8080:8080"
    depends_on:
      db:
        condition: service_healthy
    volumes:
      - ./backend:/app/backend
      - /app/backend/vendor
    working_dir: /app/backend
    command: go run cmd/server/main.go
    networks:
      - power_network

  frontend:
    build:
      context: .
      dockerfile: docker/Dockerfile.frontend
    container_name: power_monitor_frontend
    environment:
      REACT_APP_API_URL: http://localhost:8080
      NODE_ENV: development
    ports:
      - "3000:3000"
    volumes:
      - ./frontend/src:/app/frontend/src
      - ./frontend/public:/app/frontend/public
      - /app/frontend/node_modules
    depends_on:
      - backend
    networks:
      - power_network

volumes:
  postgres_data:
    driver: local

networks:
  power_network:
    driver: bridge
```

## Dockerfiles

### Backend Dockerfile

```dockerfile
// filepath: /Users/alastairhovington/Documents/Github/power-dashboard/docker/Dockerfile.backend
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY backend/go.mod backend/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY backend/ .

# Build application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /build/server .

EXPOSE 8080

CMD ["./server"]
```

### Frontend Dockerfile

```dockerfile
// filepath: /Users/alastairhovington/Documents/Github/power-dashboard/docker/Dockerfile.frontend
FROM node:18-alpine AS builder

WORKDIR /app

# Copy package files
COPY frontend/package*.json ./

# Install dependencies
RUN npm ci

# Copy source
COPY frontend/ .

# Build application
RUN npm run build

# Serve stage
FROM node:18-alpine

WORKDIR /app

# Install serve to run the app
RUN npm install -g serve

# Copy built app from builder
COPY --from=builder /app/build ./build

EXPOSE 3000

CMD ["serve", "-s", "build", "-l", "3000"]
```

## Environment Configuration

### .env.example

```bash
// filepath: /Users/alastairhovington/Documents/Github/power-dashboard/.env.example
# Backend Configuration
GO_ENV=development
LOG_LEVEL=debug
DATABASE_URL=postgres://postgres:password@localhost:5432/power_monitor

# Enphase Configuration
ENPHASE_API_KEY=your_enphase_api_key
ENPHASE_SYSTEM_ID=your_system_id

# Tesla Configuration (future)
TESLA_API_TOKEN=your_tesla_token
TESLA_SITE_ID=your_site_id

# Frontend Configuration
REACT_APP_API_URL=http://localhost:8080
NODE_ENV=development
```

## Quick Commands

```bash
# Start all services
docker-compose up

# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# View backend logs only
docker-compose logs -f backend

# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Rebuild services
docker-compose up --build

# Run command in container
docker-compose exec backend go test ./...

# Access database
docker-compose exec db psql -U postgres -d power_monitor
```

## Health Checks

Services include health checks to ensure dependencies are ready:

```yaml
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U postgres"]
  interval: 10s
  timeout: 5s
  retries: 5
```

Check service health:

```bash
docker-compose ps
docker inspect power_monitor_db --format='{{.State.Health.Status}}'
```

## Volumes

Persistent data is stored in named volumes:

- `postgres_data`: PostgreSQL database files
- Source code mounted for hot-reload development

## Networking

Services communicate on `power_network`:
- Backend connects to database at `db:5432`
- Frontend connects to backend at `backend:8080`
- Host accesses services via localhost

## Performance Optimization

### Development

```yaml
# Use alpine images for smaller size
image: postgres:14-alpine

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Multi-stage builds reduce final image size
FROM golang:1.21-alpine AS builder
FROM alpine:latest
```

### Resource Limits

```yaml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs backend

# Check network connectivity
docker-compose exec backend ping db

# Verify environment variables
docker-compose exec backend env | grep DATABASE
```

### Database Connection Error

```bash
# Ensure database is healthy
docker-compose ps

# Check database is ready
docker-compose exec db pg_isready

# Reset database
docker-compose down -v
docker-compose up
```

### Port Already in Use

```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different port in docker-compose.yml
ports:
  - "8081:8080"
```

### Hot Reload Not Working

```bash
# Restart services
docker-compose down
docker-compose up -d

# Check volume mounts
docker inspect power_monitor_backend
```

## Production Considerations

For production deployment:

1. Use environment-specific compose files: `docker-compose.prod.yml`
2. Remove volume mounts for source code
3. Add resource limits
4. Configure health checks with proper timeouts
5. Use secrets management for credentials
6. Enable logging drivers (json-file, splunk, etc.)
7. Set up monitoring and alerting

```yaml
# Production example
services:
  backend:
    image: myregistry/power-monitor:latest
    restart: always
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
```