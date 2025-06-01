# Remote Code Compiler Service

A distributed code compilation and execution service built with Go, featuring NATS messaging and Docker containerization.

## Features

- **Multi-language Support**: Supports compilation and execution of Go, C/C++, Python, Java, and Node.js
- **Distributed Architecture**: Uses NATS for message queuing and distributed processing
- **Docker Support**: Fully containerized with Docker and Docker Compose
- **Resource Limits**: Configurable time and memory limits for code execution
- **Concurrent Processing**: Supports multiple concurrent compilation jobs
- **Simple Deployment**: Easy setup with direct execution mode

## Architecture

The service consists of:

- **Runner Service**: Main Go application that processes code compilation requests
- **NATS Server**: Message broker for distributed communication
- **Direct Execution**: Code runs directly in the container environment

## Quick Start with Docker

### Prerequisites

- Docker and Docker Compose installed
- At least 2GB RAM available

### 1. Build and Run

```bash
# Clone the repository
git clone <repository-url>
cd stateless-compiler

# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f remote-compiler
```

### 2. With Monitoring (Optional)

```bash
# Start with NATS monitoring dashboard
docker-compose --profile monitoring up -d

# Access NATS monitoring at http://localhost:7777
```

### 3. Stop Services

```bash
docker-compose down

# Remove volumes (cleanup)
docker-compose down -v
```

## Configuration

### Environment Variables

| Variable                              | Default                 | Description                       |
| ------------------------------------- | ----------------------- | --------------------------------- |
| `RUNNER_NATS_URL`                     | `nats://localhost:4222` | NATS server URL                   |
| `RUNNER_RUNNER_SANDBOXBASEDIR`        | `/tmp/runner_sandbox`   | Temp directory for code execution |
| `RUNNER_RUNNER_SANDBOXTYPE`           | `direct`                | Sandbox type (direct mode)        |
| `RUNNER_RUNNER_MAXCONCURRENTJOBS`     | `20`                    | Max concurrent compilation jobs   |
| `RUNNER_RUNNER_COMPILATIONTIMEOUTSEC` | `45`                    | Compilation timeout in seconds    |

### Config File

Modify `configs/config.yaml` for additional configuration:

```yaml
nats:
  url: "nats://localhost:4222"
  submissionCreatedSubject: "submission.created"
  submissionResultSubject: "submission.result"
  queueGroup: "coderunner_prod_group"

runner:
  sandboxBaseDir: "./temp"
  compilationTimeoutSec: 45
  maxConcurrentJobs: 20
  sandboxType: "direct"
```

## Docker Images

### Building Custom Images

```bash
# Build the application
docker build -t remote-compiler .

# Build with specific Go version
docker build --build-arg GO_VERSION=1.23 -t remote-compiler .
```

### Image Details

The Docker image includes:

- **Go 1.23**: For Go compilation
- **GCC/G++**: For C/C++ compilation
- **Python 3**: For Python execution
- **OpenJDK 11**: For Java compilation
- **Node.js & NPM**: For JavaScript execution
- **Development Tools**: GDB, Valgrind, Strace

## Supported Languages

| Language   | Compiler/Runtime | Example                        |
| ---------- | ---------------- | ------------------------------ |
| Go         | `go build`       | `go build -o main main.go`     |
| C          | `gcc`            | `gcc -o main main.c`           |
| C++        | `g++`            | `g++ -o main main.cpp`         |
| Python     | `python3`        | `python3 main.py`              |
| Java       | `javac + java`   | `javac Main.java && java Main` |
| JavaScript | `node`           | `node main.js`                 |

## API Usage

The service communicates via NATS messages. Send compilation requests to the `submission.created` subject:

```json
{
  "id": "unique-submission-id",
  "language": {
    "id": "go",
    "sourceFile": "main.go",
    "binaryFile": "main",
    "compileCommand": "go build -o main main.go",
    "runCommand": "./main"
  },
  "code": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}",
  "timeLimitInMs": 2000,
  "memoryLimitInKb": 262144,
  "testCases": [
    {
      "id": "test1",
      "input": "",
      "expectOutput": "Hello, World!\n"
    }
  ]
}
```

## Monitoring

### NATS Monitoring

- **NATS Admin**: http://localhost:8222
- **NATS Surveyor**: http://localhost:7777 (with monitoring profile)

### Health Checks

```bash
# Check container health
docker-compose ps

# Check logs
docker-compose logs remote-compiler

# Manual health check
docker exec remote-compiler pgrep -f runner
```

## Development

### Local Development Setup

```bash
# Install dependencies
go mod download

# Run locally (requires NATS server)
go run cmd/runner/main.go

# Run tests
go test ./...
```

### Adding New Languages

1. Update the language configuration in your client
2. Ensure the compiler/runtime is installed in the Docker image
3. Test compilation and execution

## Troubleshooting

### Common Issues

1. **NATS Connection Failed**

   - Check if NATS container is running
   - Verify network connectivity

2. **Compilation Timeout**

   - Increase `RUNNER_RUNNER_COMPILATIONTIMEOUTSEC`
   - Check available resources

3. **Out of Memory**
   - Adjust Docker memory limits
   - Reduce concurrent jobs

### Logs

```bash
# View all logs
docker-compose logs

# Follow specific service logs
docker-compose logs -f remote-compiler
docker-compose logs -f nats
```

## Security Considerations

- Code runs in containerized environment with limited privileges
- Non-root user execution for better security
- Resource limits prevent DoS attacks
- Regular security updates of base images
- Implement rate limiting at the application level

## Performance Tuning

- Adjust `maxConcurrentJobs` based on available CPU cores
- Monitor memory usage and adjust container limits
- Use SSD storage for better I/O performance
- Consider horizontal scaling with multiple runner instances

## License

[Add your license information here]
