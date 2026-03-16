-- 001_initial_schema.down.sql
-- Drop tables in reverse dependency order.

DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS voltage_events;
DROP TABLE IF EXISTS power_quality_metrics;
DROP TABLE IF EXISTS battery_status;
DROP TABLE IF EXISTS power_readings;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS households;
DROP TABLE IF EXISTS users;
