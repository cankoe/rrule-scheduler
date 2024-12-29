# Builder stage
FROM golang:1.23.2-alpine AS builder
WORKDIR /app

# Install build dependencies if needed
# RUN apk add --no-cache build-base

# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/prequeuer ./cmd/prequeuer/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/dispatcher ./cmd/dispatcher/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker/main.go

# Final stage
FROM debian:bookworm-slim AS final
WORKDIR /app

# Copy configuration file
COPY ./config/config.yaml ./config/config.yaml

# Copy binaries
COPY --from=builder /bin/api ./api
COPY --from=builder /bin/prequeuer ./prequeuer
COPY --from=builder /bin/dispatcher ./dispatcher
COPY --from=builder /bin/worker ./worker

# Copy Swagger UI files
COPY ./swagger-ui ./swagger-ui

# Copy OpenAPI YAML file
COPY ./docs/openapi.yml ./docs/openapi.yml

# Add a non-root user for security
RUN addgroup --system app && adduser --system --ingroup app app && chown -R app:app /app
USER app

EXPOSE 8080
CMD ["./api"]