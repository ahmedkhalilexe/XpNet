# XpNet

XpNet is a lightweight **Reverse Proxy** and **Load Balancer** written in Go. This project is created for **learning purposes** to understand the fundamentals of network programming, HTTP handling, and load balancing algorithms.

## Features

- **Reverse Proxy**: Forwards client requests to one or more backend servers.
- **Load Balancing**: Distributes traffic across multiple upstream targets using a **Round Robin** strategy.
- **Prefix-Based Routing**: Routes requests based on the URL path prefix.
- **Configurable Transport**: Fine-tune HTTP connection settings like timeouts and idle connections.
- **YAML Configuration**: Easy-to-use configuration file for server settings and routes.

## How It Works

XpNet reads its configuration from a `config.yml` file located in the same directory. 

1. **Routing**: When a request arrives, XpNet searches the configured `routes` for the longest matching path prefix. Routes are sorted by prefix length to ensure the most specific match is chosen.
2. **Load Balancing**: If a route has multiple `targets`, XpNet uses an atomic counter to cycle through them (Round Robin), ensuring even distribution of requests.
3. **Request Forwarding**: The incoming request is cloned, its host and path are updated to match the target server, and it is forwarded using a performance-tuned HTTP client.
4. **Response Streaming**: The response from the upstream server, including headers and status code, is streamed back to the original client.

## Configuration

The project uses `config.yml` for all settings.

### Server Section
Defines the proxy's own behavior.
- `listen`: The port the proxy should listen on (e.g., `8080`).

### Transport Section (Optional)
Controls the HTTP client used for upstream communication.
- `max_idle_conns`: Maximum total idle connections.
- `max_idle_conns_per_host`: Maximum idle connections per host.
- `idle_conn_timeout`: Duration to keep idle connections open (e.g., `90s`).
- `timeout`: Global request timeout (e.g., `30s`).
- `disable_compression`: Boolean to disable Gzip.
- `force_http2`: Boolean to enable/disable HTTP/2.

### Routes Section
A list of path-to-target mappings.
- `path`: The URL prefix to match.
- `targets`: A list of backend server URLs. Traffic will be load balanced between these.
- `target`: (Alternative to `targets`) A single backend server URL.

## Getting Started

### Prerequisites
- Go 1.18 or higher.

### Running the Project
1. Ensure your `config.yml` is configured with your desired routes and targets.
2. Start the proxy:
   ```bash
   go run main.go
   ```

### Example `config.yml`
```yaml
server:
  listen: 8080
routes:
  - path: /api
    targets:
      - url: http://localhost:8081
        weight: 2
      - url: http://localhost:8082
```

## Future Enhancements & TODOs

To make this a fully complete production-grade reverse proxy, the following features are planned:

- **Active Health Checks**: Periodically ping upstream targets and automatically remove failing nodes from the Round Robin rotation.
- **X-Forwarded Headers**: Inject standard proxy headers (`X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host`) before forwarding requests.
- **Hop-by-Hop Headers Management**: Remove headers meant only for a single transport-level connection (e.g., `Connection`, `Keep-Alive`, `Te`, etc.).
- **Graceful Shutdown**: Intercept termination signals and finish in-flight requests before exiting.
- **Structured Logging**: Replace `fmt.Println` and `log.Println` with a leveled structured logger (like `slog` or `zap`).
- **Streaming & WebSockets Support**: Proper handling of the `Connection: Upgrade` header to support WebSockets or SSEs.
- **Retry Mechanism**: Automatically retry requests on the next available target if the current one fails or disconnects.
- **TLS/HTTPS Support**: Allow terminating SSL/TLS traffic directly at the proxy.

