# Student container image: opencode web behind the platform reverse proxy.
#
# NOTE: This is an optional example. The platform uses existing Docker images
# directly and does not require building any custom image.
#
# Runtime configuration is injected per-container by the orchestrator:
#   OPENCODE_SERVER_USERNAME / OPENCODE_SERVER_PASSWORD - basic auth (double layer)
#   OPENCODE_SERVER_BASE_PATH=/u/<username>             - base path routing
#
# Pin the base image to a fixed version so `--base-path` behavior is stable.
FROM ghcr.io/anomalyco/opencode:latest

USER root

# Common dev toolchain for students.
RUN apk add --no-cache curl git python3 py3-pip nodejs npm \
    && ln -sf /usr/bin/python3 /usr/bin/python

# Workspace is a named volume in production; make it writable by anyone.
RUN mkdir -p /workspace /root/.local/share/opencode \
    && chmod 1777 /workspace

# Baked-in opencode config (autoupdate off, server settings).
COPY opencode.json /etc/opencode/opencode.json
ENV OPENCODE_CONFIG_DIR=/etc/opencode

WORKDIR /workspace
EXPOSE 4096

CMD ["opencode", "web", "--hostname", "0.0.0.0", "--port", "4096"]
