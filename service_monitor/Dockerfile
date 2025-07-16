FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o service_monitor .

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /app/

# Create config directory
RUN mkdir -p /app/config

COPY --from=builder /app/service_monitor .
COPY --from=builder /app/config.toml /app/config/config.toml

EXPOSE 8080

# Use the working directory as the run directory
CMD ["./service_monitor"]