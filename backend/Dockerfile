# Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend .
RUN npx vite build --outDir /app/dist

# Build the API server (frontend dist is embedded at build time).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend .
COPY --from=frontend /app/dist ./internal/api/web/dist
RUN CGO_ENABLED=0 go build -trimpath -o /out/devcapsule ./cmd/server

# Final image with PostgreSQL, Nginx, and Supervisor
FROM alpine:3.20

# Install dependencies
RUN apk add --no-cache \
    postgresql16 \
    postgresql16-client \
    nginx \
    supervisor \
    ca-certificates \
    su-exec \
    bash

# Create directories
RUN mkdir -p /var/lib/postgresql/data /var/run/postgresql /var/log/supervisor && \
    chown -R postgres:postgres /var/lib/postgresql /var/run/postgresql

# Copy nginx config
COPY backend/nginx.conf /etc/nginx/http.d/default.conf

# Copy supervisor config
COPY backend/supervisord.conf /etc/supervisord.conf

# Copy the built binary
COPY --from=build /out/devcapsule /usr/local/bin/devcapsule

# Copy entrypoint script
COPY backend/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Environment variables with defaults (credentials are set at runtime in entrypoint.sh)
ENV ADDR=":8080" \
    NETWORK_NAME=devcapsule_user-net

EXPOSE 80

VOLUME ["/var/lib/postgresql/data"]

ENTRYPOINT ["/entrypoint.sh"]
