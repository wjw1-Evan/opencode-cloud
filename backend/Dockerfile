# Build the API server (frontend dist is embedded at build time).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/devcapsule ./cmd/server

# Final image with PostgreSQL, Nginx, and Supervisor
FROM alpine:3.20

# Install dependencies
RUN apk add --no-cache \
    postgresql17 \
    postgresql17-client \
    nginx \
    supervisor \
    ca-certificates \
    su-exec \
    bash

# Create directories
RUN mkdir -p /var/lib/postgresql/data /var/run/postgresql /var/log/supervisor

# Create postgres user and set ownership
RUN adduser -D -u 70 -h /var/lib/postgresql postgres && \
    chown -R postgres:postgres /var/lib/postgresql /var/run/postgresql

# Copy nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy supervisor config
COPY supervisord.conf /etc/supervisord.conf

# Copy the built binary
COPY --from=build /out/devcapsule /usr/local/bin/devcapsule

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Environment variables with defaults
ENV POSTGRES_USER=devcapsule \
    POSTGRES_PASSWORD=devcapsule \
    POSTGRES_DB=devcapsule \
    DATABASE_URL=postgres://devcapsule:devcapsule@localhost:5432/devcapsule?sslmode=disable \
    ADDR=":8080" \
    NETWORK_NAME=devcapsule_user-net

EXPOSE 80

VOLUME ["/var/lib/postgresql/data"]

ENTRYPOINT ["/entrypoint.sh"]
