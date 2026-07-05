ARG BASE_IMAGE=ubuntu:noble

#---------------------------------------------------------------------
# BUILDER IMAGE
#---------------------------------------------------------------------
FROM golang:1.22-bookworm AS builder

WORKDIR /OmniRAN-Emulator

# Copy go.mod and go.sum and download dependencies first (caching layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN cd cmd && go build -o app

#---------------------------------------------------------------------
# TARGET IMAGE
#---------------------------------------------------------------------
FROM $BASE_IMAGE AS my5grantester

# Install necessary runtime dependencies
RUN apt-get update && apt-get install -y \
    iproute2 \
    iputils-ping \
    iperf3 \
    && rm -rf /var/lib/apt/lists/*

# Set up runtime directories
WORKDIR /OmniRAN-Emulator/config
COPY --from=builder /OmniRAN-Emulator/docker/config.yml .

WORKDIR /OmniRAN-Emulator/cmd
COPY --from=builder /OmniRAN-Emulator/cmd/app .
COPY --from=builder /OmniRAN-Emulator/docker/entrypoint.sh .

# Ensure entrypoint is executable
RUN chmod +x entrypoint.sh

ENTRYPOINT ["/OmniRAN-Emulator/cmd/entrypoint.sh"]
