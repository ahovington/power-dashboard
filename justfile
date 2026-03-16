set dotenv-load

# Show available recipes
default:
    @just --list

# ── Docker ────────────────────────────────────────────────────────────────────

# Build and start all services
up:
    docker compose up --build -d

# Start without rebuilding
start:
    docker compose start

# Stop all services (preserves data)
stop:
    docker compose stop

# Stop and remove containers (preserves DB volume)
down:
    docker compose down

# Stop and remove everything including the DB volume
reset:
    docker compose down -v

# Stream logs from all services
logs:
    docker compose logs -f

# Stream logs from a specific service: just log backend
log service:
    docker compose logs -f {{service}}

# Show service status
ps:
    docker compose ps

# ── Seed ─────────────────────────────────────────────────────────────────────

# Seed 30 days of fake data (default seed=42)
seed days="30" seed="42":
    docker compose run --rm --entrypoint /app/seed backend --days={{days}} --seed={{seed}}

# ── Dev ───────────────────────────────────────────────────────────────────────

# Rebuild and restart a single service: just rebuild backend
rebuild service:
    docker compose up --build -d {{service}}

# Open a shell in a running service: just sh backend
sh service:
    docker compose exec {{service}} sh

# Run backend tests
test:
    cd backend && go test -race ./...
