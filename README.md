# Pirmon Client

### Example configuration file
```yaml
server_url: "http://127.0.0.1:7001"
server_version: "v1"
report_interval: 60 # Seconds
monitor_interval: 600 # Milliseconds
services:
  - name: "wuauserv"
    expected_status: "running"
    auto_start_if_stopped: true
    only_report: "stopped"
```
