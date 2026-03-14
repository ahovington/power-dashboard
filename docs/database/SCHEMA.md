# Database Schema

## Overview

PostgreSQL schema for storing power monitoring data, device information, and electrical metrics.

## Tables

### 1. devices

Stores information about connected devices (inverters, batteries, meters).

```sql
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id VARCHAR(50) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    device_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    serial_number VARCHAR(255) UNIQUE NOT NULL,
    location VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_devices_provider ON devices(provider_id);
CREATE INDEX idx_devices_type ON devices(device_type);
CREATE INDEX idx_devices_status ON devices(status);
```

**Fields:**
- `id`: Unique device identifier
- `provider_id`: ID from external provider (Enphase, Tesla)
- `provider_type`: Type of provider (enphase, tesla)
- `device_type`: inverter, battery, meter, load_controller
- `status`: active, inactive, fault

### 2. power_readings

Time-series data for power consumption and generation.

```sql
CREATE TABLE power_readings (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    reading_timestamp TIMESTAMP NOT NULL,
    
    -- Power metrics (in watts)
    power_produced INT,
    power_consumed INT,
    power_net INT,
    
    -- Energy metrics (in Wh)
    energy_produced_today BIGINT,
    energy_consumed_today BIGINT,
    
    -- Frequency and voltage
    frequency DECIMAL(5, 2),
    voltage_phase_a DECIMAL(6, 2),
    voltage_phase_b DECIMAL(6, 2),
    voltage_phase_c DECIMAL(6, 2),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_power_readings_device_timestamp 
    ON power_readings(device_id, reading_timestamp DESC);
CREATE INDEX idx_power_readings_timestamp 
    ON power_readings(reading_timestamp DESC);
```

**Fields:**
- `power_produced`: Solar power generation (watts)
- `power_consumed`: System power consumption (watts)
- `power_net`: Net power (production - consumption)
- `voltage_phase_*`: Three-phase voltage readings

### 3. battery_status

Battery-specific metrics and state.

```sql
CREATE TABLE battery_status (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    reading_timestamp TIMESTAMP NOT NULL,
    
    -- Battery state
    charge_percentage NUMERIC(5, 2),
    state_of_charge INT,
    state_of_health INT,
    
    -- Power flow
    power_flowing INT,
    power_direction VARCHAR(20), -- 'charging', 'discharging'
    
    -- Capacity and temperature
    capacity_wh BIGINT,
    temperature NUMERIC(5, 2),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_battery_status_device_timestamp 
    ON battery_status(device_id, reading_timestamp DESC);
```

### 4. power_quality_metrics

Electrical engineering metrics for power quality analysis.

```sql
CREATE TABLE power_quality_metrics (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    reading_timestamp TIMESTAMP NOT NULL,
    
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
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_power_quality_device_timestamp 
    ON power_quality_metrics(device_id, reading_timestamp DESC);
```

### 5. voltage_events

Tracks voltage anomalies (spikes, sags, outages).

```sql
CREATE TABLE voltage_events (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    event_timestamp TIMESTAMP NOT NULL,
    
    event_type VARCHAR(50), -- 'spike', 'sag', 'outage'
    severity VARCHAR(20), -- 'low', 'medium', 'high'
    
    -- Voltage details
    phase_affected VARCHAR(20),
    voltage_measured DECIMAL(6, 2),
    voltage_expected DECIMAL(6, 2),
    variance_percent NUMERIC(5, 2),
    
    duration_ms INT,
    acknowledged BOOLEAN DEFAULT FALSE,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_voltage_events_device_timestamp 
    ON voltage_events(device_id, event_timestamp DESC);
CREATE INDEX idx_voltage_events_severity 
    ON voltage_events(severity);
```

### 6. system_status

Overall system health and status.

```sql
CREATE TABLE system_status (
    id BIGSERIAL PRIMARY KEY,
    status_timestamp TIMESTAMP NOT NULL,
    
    -- System state
    is_online BOOLEAN,
    is_generating BOOLEAN,
    is_battery_charged BOOLEAN,
    
    -- Overall metrics
    total_power_produced INT,
    total_power_consumed INT,
    total_battery_capacity BIGINT,
    total_battery_charge BIGINT,
    
    -- Alerts
    alert_count INT DEFAULT 0,
    critical_alerts INT DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_system_status_timestamp 
    ON system_status(status_timestamp DESC);
```

### 7. alerts

System alerts and notifications.

```sql
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID REFERENCES devices(id),
    
    alert_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20), -- 'info', 'warning', 'critical'
    message TEXT,
    
    is_resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_alerts_device_severity 
    ON alerts(device_id, severity);
CREATE INDEX idx_alerts_resolved 
    ON alerts(is_resolved);
CREATE INDEX idx_alerts_timestamp 
    ON alerts(created_at DESC);
```

### 8. api_credentials

Stores encrypted API credentials for providers.

```sql
CREATE TABLE api_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_type VARCHAR(50) NOT NULL,
    
    key_name VARCHAR(255) NOT NULL,
    encrypted_value TEXT NOT NULL,
    
    is_active BOOLEAN DEFAULT TRUE,
    last_used TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(provider_type, key_name)
);
```

## Relationships

```
devices
    ├── power_readings (1:N)
    ├── battery_status (1:N)
    ├── power_quality_metrics (1:N)
    └── voltage_events (1:N)

api_credentials (N:1 relationship via provider_type)
```

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

3. **Get all active devices**
   ```sql
   SELECT * FROM devices WHERE status = 'active';
   ```
   Index: `(status)`

4. **Get unresolved alerts**
   ```sql
   SELECT * FROM alerts 
   WHERE is_resolved = FALSE 
   ORDER BY created_at DESC;
   ```
   Index: `(is_resolved, created_at DESC)`

## Time-Series Considerations

### Data Retention

- **Power Readings**: Keep 1 year of detailed data (5-min intervals)
- **Aggregated Data**: Keep indefinite daily/weekly summaries
- **Alerts**: Keep 6 months

### Partitioning Strategy

For large datasets, consider table partitioning:

```sql
CREATE TABLE power_readings_2024_01 PARTITION OF power_readings
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
```

### Compression

Archive old data to cold storage:

```sql
-- Archive old readings
INSERT INTO power_readings_archive
SELECT * FROM power_readings 
WHERE reading_timestamp < NOW() - INTERVAL '90 days';

DELETE FROM power_readings 
WHERE reading_timestamp < NOW() - INTERVAL '90 days';
```

## Performance Optimization

### Connection Pooling

- Min connections: 5
- Max connections: 20
- Idle timeout: 30 seconds

### Query Optimization

1. Always filter by device_id and timestamp together
2. Use LIMIT for pagination
3. Aggregate data at application layer
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
- `TIMESTAMP`: Date and time with timezone
- `TEXT`: Variable-length text
- `BOOLEAN`: True/false

## Future Enhancements

1. Time-series specific database (InfluxDB, TimescaleDB)
2. Data warehouse for analytics (BigQuery, Snowflake)
3. Real-time streaming (Kafka, RabbitMQ)
4. GraphQL API with subscriptions for real-time updates