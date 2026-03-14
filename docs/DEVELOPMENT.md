# Development Guide

## Prerequisites

- Go 1.21+
- Node.js 18+
- Docker & Docker Compose
- PostgreSQL 14+ (via Docker)

## Setup

### 1. Clone Repository

```bash
git clone https://github.com/yourusername/power-dashboard.git
cd power-dashboard
```

### 2. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` with your credentials:

```
GO_ENV=development
DATABASE_URL=postgres://postgres:password@localhost:5432/power_monitor
ENPHASE_API_KEY=your_key
ENPHASE_SYSTEM_ID=your_system_id
REACT_APP_API_URL=http://localhost:8080
```

### 3. Start Services

```bash
docker-compose up -d
```

This starts:
- PostgreSQL database
- Go backend (port 8080)
- React dev server (port 3000)

### 4. Initialize Database

```bash
docker-compose exec backend go run cmd/server/main.go migrate
```

## Development Workflow

### Backend Development

```bash
# Watch for changes and rebuild
docker-compose exec backend air

# Run tests
docker-compose exec backend go test ./...

# Run specific test
docker-compose exec backend go test -run TestPowerService ./...

# Format code
docker-compose exec backend go fmt ./...

# Lint code
docker-compose exec backend golangci-lint run
```

### Frontend Development

```bash
# In another terminal
cd frontend
npm install
npm start
```

Frontend hot-reloads automatically on file changes.

## Testing

### Backend Tests

```bash
# Unit tests
go test ./internal/...

# With coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Frontend Tests

```bash
cd frontend
npm test
npm run test:coverage
```

## Code Structure

Follow these conventions:

### Go Code

- Package names: lowercase, one word
- Function names: CamelCase, exported functions start with uppercase
- Interfaces: end with "er" suffix (Reader, Writer, Adapter)
- Error handling: explicit, no silent failures
- Comments: document exported functions

### React Code

- Component names: PascalCase
- Hook names: start with "use"
- File names: match component/hook names
- Props: define with TypeScript interfaces
- State: use hooks, prefer useReducer for complex state

## Git Workflow

```bash
# Create feature branch
git checkout -b feature/new-feature

# Make changes and commit
git commit -m "feat: add new feature"

# Push branch
git push origin feature/new-feature

# Create Pull Request on GitHub
```

### Commit Messages

Use conventional commits:
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation
- `style:` Code formatting
- `refactor:` Code refactoring
- `test:` Adding tests
- `chore:` Build/tooling changes

## Debugging

### Backend

```bash
# Enable debug logging
GO_LOG=debug docker-compose up

# Use debugger with Delve
dlv debug ./cmd/server/
```

### Frontend

- Use React DevTools Chrome extension
- Use Redux DevTools for state debugging
- Browser DevTools for network inspection

## Common Tasks

### Add New Database Migration

```bash
# Create migration file
touch backend/migrations/002_add_column.sql

# Run migrations
docker-compose exec backend go run cmd/server/main.go migrate
```

### Add New API Endpoint

1. Create handler in `internal/api/handler.go`
2. Add route in `internal/api/routes.go`
3. Add service method if needed
4. Write tests
5. Document in API_ENDPOINTS.md

### Add New React Component

```bash
cd frontend/src/components
# Create component file
touch NewComponent.tsx
```

## Troubleshooting

### Database Connection Issues

```bash
docker-compose logs db
docker-compose exec db psql -U postgres -d power_monitor
```

### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>
```

### Hot Reload Not Working

```bash
# Restart Docker services
docker-compose down
docker-compose up -d
```

## Resources

- [Go Documentation](https://golang.org/doc/)
- [React Documentation](https://react.dev/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Docker Documentation](https://docs.docker.com/)