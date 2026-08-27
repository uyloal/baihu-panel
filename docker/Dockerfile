# ================================
# Stage 1: Build frontend
# ================================
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder

WORKDIR /app/web

RUN corepack enable && corepack prepare pnpm@11.24.0 --activate

# Copy package files
COPY web/package*.json web/pnpm-lock.yaml* ./

# Install frontend dependencies
RUN pnpm install --frozen-lockfile=false

# Copy frontend source
COPY web/ ./

# Build frontend
RUN pnpm run build

# ================================
# Stage 0: Generate build info
# ================================
FROM alpine:3.21 AS build-info

ARG VERSION
ARG BUILD_TIME

RUN VERSION_VAL="${VERSION:-dev-$(TZ=Asia/Shanghai date '+%Y%m%d%H%M%S')}" && \
    BUILD_TIME_VAL="${BUILD_TIME:-$(TZ=Asia/Shanghai date '+%Y-%m-%d %H:%M:%S')}" && \
    mkdir -p /build-info && \
    echo "${VERSION_VAL}" > /build-info/version.txt && \
    echo "${BUILD_TIME_VAL}" > /build-info/build_time.txt

# ================================
# Stage 2: Build backend
# ================================
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS backend-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy build info
COPY --from=build-info /build-info /build-info

# Go mod files
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy backend source
COPY . .

# Copy frontend dist (embedded into binary)
COPY --from=frontend-builder /app/web/dist ./internal/static/dist

# Build Go binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    VERSION_VAL=$(cat /build-info/version.txt) && \
    BUILD_TIME_VAL=$(cat /build-info/build_time.txt) && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X github.com/uyloal/baihu-panel/internal/constant.Version=${VERSION_VAL} -X 'github.com/uyloal/baihu-panel/internal/constant.BuildTime=${BUILD_TIME_VAL}'" \
    -o baihu .

# ================================
# Stage 3: Final image (Node.js LTS 24 + pnpm 11.24 on Alpine)
# ================================
FROM node:24-alpine

ENV TZ=Asia/Shanghai
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV PNPM_HOME=/root/.pnpm
ENV PATH="/app/data/node_modules/.bin:$PNPM_HOME/bin:$PNPM_HOME:/usr/local/bin:$PATH"

# Install essential runtime tools
RUN apk add --no-cache \
    bash \
    git \
    curl \
    ca-certificates \
    tzdata \
    rsync \
    openssh-client \
    && echo "${TZ}" > /etc/timezone \
    && ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime \
    && corepack enable \
    && corepack prepare pnpm@11.24.0 --activate \
    && pnpm config set global-bin-dir /root/.pnpm/bin

WORKDIR /app

# Copy Go binary
COPY --from=backend-builder /app/baihu /app/baihu
RUN ln -sf /app/baihu /usr/local/bin/baihu

# Copy frontend assets to /www/baihu
COPY --from=frontend-builder /app/web/dist /www/baihu

# Copy configs, SDK package, example scripts, and entrypoint
COPY configs ./configs
COPY example ./example
COPY packages/baihu /app/packages/baihu
COPY docker/docker-entrypoint.sh /app/docker-entrypoint.sh

RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 8052

ENTRYPOINT ["/app/docker-entrypoint.sh"]
