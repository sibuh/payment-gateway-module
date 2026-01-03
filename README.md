
## ✨ Features

- RESTful API for payment processing
- Support for multiple currencies (USD, ETB)
- Asynchronous payment processing with RabbitMQ
- Input validation and error handling
- Retry mechanism for failed operations
- Containerized with Docker
- Comprehensive test coverage

## 🚀 Prerequisites

- Go 1.24.0
- PostgreSQL 13+
- RabbitMQ 3.8+
- Docker 

## 🛠️ Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/sibuh/pgm
   cd pgm
   ```

2. Copy the example environment file and update the values:
   ```bash
   cp .env.example .env
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Start the services using Docker Compose:
   ```bash
   docker-compose up -d
   ```

## 📚 API Documentation

### Create a Payment

```http
POST /v1/payments
Content-Type: application/json

{
  "amount": 100.50,
  "currency": "USD",
  "reference": "order-123"
}
```

### Get Payment by ID

```http
GET /v1/payments/{payment_id}
```

## 🧪 Running Tests

To run all tests:

```bash
go test -v ./...
```

## 🐳 Docker Support

The project includes Dockerfiles for both the API and worker services. To build and run the containers:

```bash
# Build the containers
docker-compose up --build -d

# View logs
docker-compose logs -f
```

## 📁 Project Structure

```
.
├── api/                  # api server entry point
|── worker/               # worker server entry point
|                        
├── internal/
│   ├── domain/           # Domain models and interfaces
│   ├── handler/          # HTTP handlers
│   ├── service/          # Business logic
│   └── queue/            # Message queue handlers
├── migrations/           # Database migrations
├── .env.example          # Example environment variables
├── docker-compose.yml    # Docker Compose configuration
├── Dockerfile.api        # API service Dockerfile
└── Dockerfile.worker     # Worker service Dockerfile
```


