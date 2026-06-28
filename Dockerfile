# Multi-stage build for the Spectra server + setup + seed binaries.
# The frontend is built first and embedded into the server via go:embed
# (web/embed.go), so the final image is a single static-ish binary plus the
# setup and seed tools used by the compose entrypoint.

# Stage 1: frontend assets
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build
# produces web/dist, consumed by go:embed in the next step

# Stage 2: go binaries
FROM golang:1.26-alpine AS build
WORKDIR /src

# Module cache layer
COPY go.mod go.sum ./
RUN go mod download

# Source + built frontend
COPY . .
COPY --from=web /app/web/dist ./web/dist

# Version stamping mirrors the Makefile's BASE_LDFLAGS. These are passed in by
# compose as build args; default keep a standalone `docker build` working.
ARG VERSION=0.0.0-docker
ARG COMMIT=unknown
ARG DATE=unknown
ENV LDFLAGS="-s -w \
	-X github.com/nhdewitt/spectra/internal/version.Version=${VERSION} \
	-X github.com/nhdewitt/spectra/internal/version.Commit=${COMMIT} \
	-X github.com/nhdewitt/spectra/internal/version=Date=${DATE}"

RUN go build -ldflags "${LDFLAGS}" -trimpath -o /out/spectra-server ./cmd/server \
 && go build -ldflags "${LDFLAGS}" -trimpath -o /out/spectra-setup ./cmd/setup \
 && go build -ldflags "${LDFLAGS}" -trimpath -o /out/spectra-seed ./cmd/seed

# Stage 3: runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app

RUN mkdir -p /etc/spectra

COPY --from=build /out/spectra-server /usr/local/bin/spectra-server
COPY --from=build /out/spectra-setup /usr/local/bin/spectra-setup
COPY --from=build /out/spectra-seed /usr/local/bin/spectra-seed

COPY internal/database/migrations ./internal/database/migrations