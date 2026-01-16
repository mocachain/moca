FROM golang:1.23.6-bullseye AS builder

ENV CGO_CFLAGS="-O -D__BLST_PORTABLE__"
ENV CGO_CFLAGS_ALLOW="-O -D__BLST_PORTABLE__"

ENV GOPRIVATE=github.com/mocachain

ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:@github.com/".insteadOf "https://github.com/"

WORKDIR /workspace

COPY . .
RUN make build


FROM golang:1.23.6-bullseye
RUN apt-get update -y && apt-get install ca-certificates jq -y

# Copy binary to a location that won't be shadowed by volume mounts
COPY --from=builder /workspace/build/mocad /usr/local/bin/mocad

WORKDIR /root

# Install Cosmovisor
RUN go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@v1.7.1

# Environment variables for Cosmovisor
ENV DAEMON_NAME=mocad
ENV DAEMON_HOME=/root/.mocad
ENV DAEMON_ALLOW_DOWNLOAD_BINARIES=true
ENV DAEMON_RESTART_AFTER_UPGRADE=true
ENV DAEMON_POLL_INTERVAL=1s

# Copy entrypoint script
# This script will automatically copy the genesis binary to the daemon home
# directory if it doesn't exist.
COPY cosmovisor-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["cosmovisor"]