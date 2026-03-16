# Household Power Monitor

A comprehensive energy monitoring application that integrates with solar inverters and battery systems to track power consumption, generation, and electrical metrics in real-time.

## Features

- **Real-time Power Monitoring**: Track consumption, generation, and net power flow
- **Solar Generation Tracking**: Monitor solar panel output and efficiency
- **Battery Status**: Display battery charge level, power flow, and health metrics
- **Power Quality Analysis**: Monitor voltage, current, power factor, and harmonic distortion
- **Multi-Provider Support**: Built with extensibility for multiple API providers (Enphase, Tesla, etc.)
- **Historical Data**: View trends and patterns over time
- **Responsive Dashboard**: Real-time updates with beautiful visualizations

## Tech Stack

- **Backend**: Go with clean architecture
- **Frontend**: React with TypeScript
- **Database**: PostgreSQL with time-series optimization
- **Infrastructure**: Docker Compose for local development
- **Testing**: TDD approach with mock APIs

## Quick Start

### Prerequisites

- [Docker & Docker Compose](https://docs.docker.com/get-docker/)
- [just](https://github.com/casey/just) — command runner (`brew install just`)
- Git

### Setup

```bash
git clone https://github.com/ahovington/power-dashboard.git
cd power-dashboard
cp .env.example .env   # edit values as needed
just up                # build images and start all services
just seed              # seed 30 days of fake solar data
```

Access the application at `http://localhost`.

### Common commands

| Command | Description |
|---|---|
| `just up` | Build and start all services |
| `just stop` | Stop services (data preserved) |
| `just down` | Remove containers (data preserved) |
| `just reset` | Remove containers + DB volume |
| `just logs` | Tail logs from all services |
| `just log backend` | Tail logs from one service |
| `just seed` | Seed 30 days of fake data (seed=42) |
| `just seed 7 99` | Seed 7 days with a custom seed |
| `just rebuild backend` | Rebuild and restart one service |
| `just test` | Run backend test suite |

### Fake / demo data

To run without real Enphase credentials, set in `.env`:

```
FAKE_PROVIDER=true
FAKE_SEED=42   # omit or set to 0 for non-deterministic data
```

Then seed historical data so the history charts have data to display:

```bash
just seed        # 30 days, deterministic seed=42
just seed 90 0   # 90 days, random seed
```

### Environment Variables

Copy `.env.example` to `.env` and fill in your values. Key variables:

```
DATABASE_URL=postgres://postgres:changeme@db:5432/power_monitor?sslmode=disable
ENPHASE_API_KEY=your_enphase_key   # leave blank to use fake provider
ENPHASE_SYSTEM_ID=your_system_id
FAKE_PROVIDER=false
FAKE_SEED=0
```

## Project Structure

```
power-dashboard/
├── backend/              # Go backend application
├── frontend/             # React frontend application
├── docker/              # Docker configurations
├── docs/                # Documentation
└── docker-compose.yml   # Local development setup
```

## Documentation

- [Backend Implementation Guide](./docs/backend/IMPLEMENTATION.md)
- [API Integration Guide](./docs/backend/api/INTEGRATION.md)
- [Database Schema](./docs/backend/database/SCHEMA.md)

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License.

## Support

For issues and questions, please open an issue on GitHub.