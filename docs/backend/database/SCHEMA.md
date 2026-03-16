# Database Schema

## Overview

PostgreSQL schema for storing power monitoring data, device information, and electrical metrics.

## Tables

### 1. users

Stores user accounts. Required for household ownership and future multi-household support.

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
```

### 2. households

A household owns devices and provider credentials.

```sql
CREATE TABLE households (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    timezone VARCHAR(100) NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_households_user ON households(user_id);
```

### 3. devices

Stores information about connected devices (inverters, batteries, meters).

```sql
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id),
    provider_id VARCHAR(50) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    device_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    serial_number VARCHAR(255) UNIQUE NOT NULL,
    location VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_devices_household ON devices(household_id);
CREATE INDEX idx_devices_provider ON devices(provider_id);
CREATE INDEX idx_devices_type ON devices(device_type);
CREATE INDEX idx_devices_status ON devices(status);
```

**Fields:**
- `household_id`: Owning household
- `provider_id`: ID from external provider (Enphase, Tesla)
- `provider_type`: Type of provider (enphase, tesla)
- `device_type`: inverter, battery, meter, load_controller
- `status`: active, inactive, fault

### 4. power_readings

Time-series data for power consumption and generation.

```sql
CREATE TABLE power_readings (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    reading_timestamp TIMESTAMPTZ NOT NULL,

    -- Power metrics (in watts)
    power_produced INT,
    power_consumed INT,
    -- power_net is computed on read as (power_produced - power_consumed)

    -- Energy metrics (in Wh)
    energy_produced_today BIGINT,
    energy_consumed_today BIGINT,

    -- Frequency and voltage
    frequency DECIMAL(5, 2),
    voltage_phase_a DECIMAL(6, 2),
    voltage_phase_b DECIMAL(6, 2),
    voltage_phase_c DECIMAL(6, 2),

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(device_id, reading_timestamp)
);

CREATE INDEX idx_power_readings_device_timestamp
    ON power_readings(device_id, reading_timestamp DESC);
CREATE INDEX idx_power_readings_timestamp
    ON power_readings(reading_timestamp DESC);
```

**Fields:**
- `power_produced`: Solar power generation (watts)
- `power_consumed`: System power consumption (watts)
- `voltage_phase_*`: Three-phase voltage readings

> **Note:** `power_net` is not stored. Compute it on read as `power_produced - power_consumed` to avoid consistency drift.
>
> **Note:** `UNIQUE(device_id, reading_timestamp)` ensures re-polling the same window on restart does not create duplicate rows. Use `INSERT ... ON CONFLICT DO NOTHING` in the repository.

### 5. battery_status

Battery-specific metrics and state.

```sql
CREATE TABLE battery_status (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    reading_timestamp TIMESTAMPTZ NOT NULL,

    -- Battery state
    charge_percentage NUMERIC(5, 2),  -- 0.00–100.00
    state_of_health INT,

    -- Power flow
    power_flowing INT,
    power_direction VARCHAR(20), -- 'charging', 'discharging'

    -- Capacity and temperature
    capacity_wh BIGINT,
    temperature NUMERIC(5, 2),

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(device_id, reading_timestamp)
);

CREATE INDEX idx_battery_status_device_timestamp
    ON battery_status(device_id, reading_timestamp DESC);
```

> **Note:** Only `charge_percentage NUMERIC(5,2)` is kept. The redundant `state_of_charge INT` column has been removed to avoid precision drift between two representations of the same value.

### 6. power_quality_metrics

Electrical engineering metrics for power quality analysis.

```sql
CREATE TABLE power_quality_metrics (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    reading_timestamp TIMESTAMPTZ NOT NULL,

    -- Power factor
    power_factor_phase_a NUMERIC(4, 3),
    power_factor_phase_b NUMERIC(4, 3),
    power_factor_phase_c NUMERIC(4, 3),
    power_factor_average NUMERIC(4, 3),

    -- Current (amperes)
    current_phase_a NUMERIC(6, 2),
    current_phase_b NUMERIC(6, 2),
    current_phase_c NUMERIC(6, 2),

    -- Harmonic distortion
    thd_voltage_phase_a NUMERIC(5, 2),
    thd_voltage_phase_b NUMERIC(5, 2),
    thd_voltage_phase_c NUMERIC(5, 2),
    thd_current_phase_a NUMERIC(5, 2),
    thd_current_phase_b NUMERIC(5, 2),
    thd_current_phase_c NUMERIC(5, 2),

    -- Reactive power
    reactive_power_phase_a INT,
    reactive_power_phase_b INT,
    reactive_power_phase_c INT,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(device_id, reading_timestamp)
);

CREATE INDEX idx_power_quality_device_timestamp
    ON power_quality_metrics(device_id, reading_timestamp DESC);
```

### 7. voltage_events

Tracks voltage anomalies (spikes, sags, outages).

```sql
CREATE TABLE voltage_events (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    event_timestamp TIMESTAMPTZ NOT NULL,

    event_type VARCHAR(50), -- 'spike', 'sag', 'outage'
    severity VARCHAR(20), -- 'low', 'medium', 'high'

    -- Voltage details
    phase_affected VARCHAR(20),
    voltage_measured DECIMAL(6, 2),
    voltage_expected DECIMAL(6, 2),
    variance_percent NUMERIC(5, 2),

    duration_ms INT,
    acknowledged BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_voltage_events_device_timestamp
    ON voltage_events(device_id, event_timestamp DESC);
CREATE INDEX idx_voltage_events_severity
    ON voltage_events(severity);
```

### 8. alerts

System alerts and notifications.

```sql
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID REFERENCES devices(id),

    alert_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20), -- 'info', 'warning', 'critical'
    message TEXT,

    is_resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_alerts_device_severity
    ON alerts(device_id, severity);
CREATE INDEX idx_alerts_resolved
    ON alerts(is_resolved);
CREATE INDEX idx_alerts_timestamp
    ON alerts(created_at DESC);
```

## Provider Credentials

Provider API keys (Enphase, Tesla, etc.) are stored exclusively in environment variables — not in the database. This avoids the encryption key management complexity of a database-stored credentials table.

Required env vars per provider:

```bash
# Enphase
ENPHASE_API_KEY=...
ENPHASE_SYSTEM_ID=...

# Tesla (future)
TESLA_API_TOKEN=...
TESLA_SITE_ID=...
```

## System Status

There is no `system_status` table. System-wide totals (total production, total consumption, overall battery level) are computed at the service layer by aggregating the latest reading per device. This avoids consistency drift between a summary table and the source time-series tables.

## Relationships

```
users
    └── households (1:N)
            └── devices (1:N)
                    ├── power_readings (1:N)
                    ├── battery_status (1:N)
                    ├── power_quality_metrics (1:N)
                    └── voltage_events (1:N)
```

## Timezone Handling

All timestamp columns use `TIMESTAMPTZ` (timestamp with time zone). This is critical for correctness across DST boundaries (e.g. AEST/AEDT in Australia). The application layer stores timestamps as UTC; the frontend displays in the household's configured timezone.

## Indexing Strategy

### Query Patterns

1. **Get latest readings for a device**
   ```sql
   SELECT * FROM power_readings
   WHERE device_id = $1
   ORDER BY reading_timestamp DESC
   LIMIT 100;
   ```
   Index: `(device_id, reading_timestamp DESC)`

2. **Get readings for time range**
   ```sql
   SELECT * FROM power_readings
   WHERE device_id = $1
   AND reading_timestamp BETWEEN $2 AND $3;
   ```
   Index: `(device_id, reading_timestamp DESC)`

3. **Aggregated data for charts (daily averages)**
   ```sql
   SELECT
       DATE_TRUNC('hour', reading_timestamp) AS hour,
       AVG(power_produced) AS avg_produced,
       SUM(energy_produced_today) AS total_produced
   FROM power_readings
   WHERE device_id = $1
   AND reading_timestamp BETWEEN $2 AND $3
   GROUP BY 1
   ORDER BY 1;
   ```
   Use `DATE_TRUNC` + aggregation rather than returning raw rows to the application layer.

4. **Get all active devices for a household**
   ```sql
   SELECT * FROM devices WHERE household_id = $1 AND status = 'active';
   ```
   Index: `(household_id)`

5. **Get unresolved alerts**
   ```sql
   SELECT * FROM alerts
   WHERE is_resolved = FALSE
   ORDER BY created_at DESC;
   ```
   Index: `(is_resolved, created_at DESC)`

## Duplicate Prevention

The background ingestion goroutine polls providers at regular intervals. On restart, it may re-read an overlapping time window. The `UNIQUE(device_id, reading_timestamp)` constraint on time-series tables prevents duplicate rows. All repository insert methods use:

```sql
INSERT INTO power_readings (...) VALUES (...)
ON CONFLICT (device_id, reading_timestamp) DO NOTHING;
```

## Time-Series Considerations

### Data Retention

- **Power Readings**: Keep 1 year of detailed data (5-min intervals ≈ 105,000 rows/device/year)
- **Aggregated Data**: Keep indefinite daily/weekly summaries
- **Alerts**: Keep 6 months

### Archival

Archive old data periodically:

```sql
-- Archive readings older than 90 days
INSERT INTO power_readings_archive
SELECT * FROM power_readings
WHERE reading_timestamp < NOW() - INTERVAL '90 days';

DELETE FROM power_readings
WHERE reading_timestamp < NOW() - INTERVAL '90 days';
```

Run as a scheduled task (cron or application-layer ticker). Partitioning can be added later if the table grows beyond ~1M rows.

## Performance Optimization

### Connection Pooling

- Min connections: 5
- Max connections: 20
- Idle timeout: 30 seconds

### Query Optimization

1. Always filter by `device_id` and `reading_timestamp` together
2. Use `LIMIT` for pagination
3. Use `DATE_TRUNC` + `AVG/SUM` for chart data — never return raw rows for time-range queries
4. Use prepared statements to prevent SQL injection

### Monitoring

```sql
-- Check slow queries
SELECT query, calls, mean_time
FROM pg_stat_statements
WHERE mean_time > 1000
ORDER BY mean_time DESC;

-- Check table sizes
SELECT schemaname, tablename,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename))
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

## Backup Strategy

### Daily Backups

```bash
pg_dump power_monitor > backup_$(date +%Y%m%d).sql
```

### Point-in-time Recovery

Enable WAL archiving for PITR capability.

## Data Types Reference

- `UUID`: Universally unique identifier
- `BIGSERIAL`: 64-bit auto-increment
- `NUMERIC(p,s)`: Decimal with precision and scale
- `TIMESTAMPTZ`: Timestamp with time zone (always use this — never plain `TIMESTAMP`)
- `TEXT`: Variable-length text
- `BOOLEAN`: True/false

## Future Enhancements

1. Time-series specific database (TimescaleDB) for continuous aggregates and automatic data retention policies
2. Table partitioning by month once `power_readings` exceeds ~1M rows
3. Data warehouse for analytics (BigQuery, Snowflake)
4. Real-time streaming (Kafka, RabbitMQ) for multi-site deployments
