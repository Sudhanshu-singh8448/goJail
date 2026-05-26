# ─── Stage 1: Build nsjail and install language toolchains ───
FROM debian:bookworm-slim AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    gnupg \
    build-essential \
    wget \
    git \
    unzip \
    protobuf-compiler \
    libprotobuf-dev \
    flex \
    bison \
    pkg-config \
    libnl-3-dev \
    libnl-route-3-dev \
    libnl-genl-3-dev \
    libnl-nf-3-dev

# Build nsjail from source (pinned to tag 3.4)
RUN git clone --branch 3.4 --depth 1 https://github.com/google/nsjail.git /opt/nsjail-src \
    && cd /opt/nsjail-src \
    && make \
    && cp /opt/nsjail-src/nsjail /usr/bin/nsjail \
    && rm -rf /opt/nsjail-src

# Install language toolchains
COPY scripts/ /tmp/scripts/
RUN chmod +x /tmp/scripts/lang_install/*.sh

# C and C++ (gcc/g++ come from build-essential)
RUN bash /tmp/scripts/lang_install/c.sh
RUN bash /tmp/scripts/lang_install/cpp.sh

# Python 3
RUN bash /tmp/scripts/lang_install/python3.sh

# Java
RUN bash /tmp/scripts/lang_install/java.sh

# Node.js
RUN bash /tmp/scripts/lang_install/javascript.sh

# Bash (already present, just verify)
RUN bash /tmp/scripts/lang_install/bash.sh

# Verilog
RUN bash /tmp/scripts/lang_install/verilog.sh

RUN apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/scripts

# ─── Stage 2: Build the Go binary ───
FROM golang:1.23-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X github.com/nsjail-server/gojail/internal/api.Version=${VERSION} \
              -X github.com/nsjail-server/gojail/internal/api.Commit=${COMMIT}" \
    -o /build/goboxd ./cmd/server/

# ─── Stage 3: Final image ───
FROM base AS final

WORKDIR /app

COPY --from=builder /build/goboxd /app/goboxd
COPY config/ /app/config/
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Create jail directory
RUN mkdir -p /tmp/goboxd_jails

EXPOSE 8000

ENTRYPOINT ["/bin/bash", "/app/entrypoint.sh"]
CMD ["/app/goboxd"]
