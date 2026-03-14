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
- Docker & Docker Compose
- Git

### Setup

```bash
git clone https://github.com/yourusername/power-dashboard.git
cd power-dashboard
docker-compose up -d
```

Access the application at `http://localhost:3000`

### Environment Variables

Create a `.env` file in the root directory:

```
# Backend
GO_ENV=development
DATABASE_URL=postgres://postgres:password@db:5432/power_monitor
ENPHASE_API_KEY=your_enphase_key
ENPHASE_SYSTEM_ID=your_system_id

# Frontend
REACT_APP_API_URL=http://localhost:8080
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

- [Architecture Design](./docs/architecture/DESIGN.md)
- [API Integration Guide](./docs/api/INTEGRATION.md)
- [Database Schema](./docs/database/SCHEMA.md)
- [Development Guide](./docs/DEVELOPMENT.md)

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