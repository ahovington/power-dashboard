-- 001_initial_schema.up.sql
-- Full schema for power monitoring.
-- All timestamps use TIMESTAMPTZ (never plain TIMESTAMP) for DST correctness.

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);

CREATE TABLE households (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    timezone VARCHAR(100) NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_households_user ON households(user_id);

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

CREATE INDEX idx_alerts_device_severity ON alerts(device_id, severity);
CREATE INDEX idx_alerts_resolved ON alerts(is_resolved);
CREATE INDEX idx_alerts_timestamp ON alerts(created_at DESC);
