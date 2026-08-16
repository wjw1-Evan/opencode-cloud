# Build the API server (frontend dist is embedded at build time).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/devcapsule ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/devcapsule /usr/local/bin/devcapsule
# Runs as root: the container must be able to read the mounted host docker.sock
# (group ownership varies across hosts).
EXPOSE 8080
ENTRYPOINT ["devcapsule"]
