# Incident Management with Prometheus and Alertmanager

This repository contains a Docker Compose setup for Prometheus and Alertmanager to monitor and alert on service issues.

## Components

- **Prometheus**: Metrics collection and monitoring system (port 9090)
- **Alertmanager**: Alert handling and notification system (port 9093)
- **Service Monitor**: Sample Go service exposing metrics (port 8080)

## Usage

1. Start the monitoring stack:
   ```
   docker-compose up -d
   ```

2. Access the services:
   - Prometheus UI: http://localhost:9090
   - Alertmanager UI: http://localhost:9093
   - Service Monitor: http://localhost:8080

3. The service_monitor is a simple Go application that:
   - Exposes Prometheus metrics at /metrics
   - Simulates random processing times
   - Randomly generates errors (10% of the time)
   - Provides metrics for requests, duration, active connections, and error rate
   - Tracks service status via `service_monitor_up{service="service_name"}` metrics
   - Monitors a config.toml file for service status changes

## Configuration Files

- `prometheus/prometheus.yml`: Prometheus configuration with scrape targets
- `prometheus/rules/alert.yml`: Example alert rule for service monitoring
- `alertmanager/alertmanager.yml`: Alertmanager configuration with notification settings

## Extending

To add more monitoring targets, edit the `prometheus/prometheus.yml` file and add new entries under `scrape_configs`.

To add more alert rules, create new YAML files in the `prometheus/rules/` directory.

## Service Status Monitoring

The `service_monitor` application reads service status from a TOML configuration file:

```toml
# Service Monitor Configuration

# Services that are currently up
up_services = [
  "api-gateway",
  "auth-service",
  "user-service"
]

# Services that are currently down
down_services = [
  "notification-service"
]
```

To update service status:

You can directly edit the configuration file since it's stored in a Docker volume. For easier access, let's modify the docker-compose.yml to use a local directory instead of a named volume:

```yaml
services:
  service_monitor:
    # ... existing configuration ...
    volumes:
      - ./config:/app/config  # Mount local directory instead of volume

# Remove service_config from volumes section if you make this change
```

With this change, you can simply:
1. Edit `./config/config.toml` directly with any text editor
2. Save the file - changes will be detected within 3 seconds

The service_monitor automatically watches for file changes and will update metrics immediately when you save the file.

The service_monitor will automatically detect changes (within 3 seconds) and update the Prometheus metrics. Each service will have a metric `service_monitor_up{service="service_name"}` with a value of:
- `1` for services in the up_services list
- `0` for services in the down_services list

You can view the current configuration at http://localhost:8080/config