# syntax=docker/dockerfile:1
FROM golang:1.27.4 AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY api api
COPY pkg pkg
COPY apps/proxy-control/cmd apps/proxy-control/cmd
COPY apps/proxy-control/internal apps/proxy-control/internal
COPY apps/proxy-gateway/cmd apps/proxy-gateway/cmd
COPY apps/proxy-gateway/internal apps/proxy-gateway/internal
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/proxy-control ./apps/proxy-control/cmd && \
    CGO_ENABLED=0 go build -o /out/proxy-gateway ./apps/proxy-gateway/cmd

FROM golang:1.27.4 AS goose-build
ARG GOOSE_VERSION=v3.30.0
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go install -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

FROM golang:1.27.4 AS gost-download
RUN curl --fail --silent --show-error --location --retry 3 \
    https://github.com/go-gost/gost/releases/download/v3.3.3/gost_3.3.3_linux_amd64.tar.gz -o /tmp/gost.tar.gz && \
    echo 'f4c1255dc892a7014c0ad6e5f1c3573108f7bea4403f04f21873406ab58aa666  /tmp/gost.tar.gz' | sha256sum -c - && \
    tar -xzf /tmp/gost.tar.gz -C /tmp gost

FROM alpine:3.23 AS control
COPY --from=go-build /out/proxy-control /usr/local/bin/
COPY deploy/test/init.yaml /config/init.yaml
COPY deploy/test/control.yaml /config/server.yaml
ENTRYPOINT ["proxy-control", "-conf", "/config/init.yaml"]

FROM alpine:3.23 AS wukong-mock
COPY --from=gost-download /tmp/gost /usr/local/bin/
ENTRYPOINT ["gost"]

FROM wukong-mock AS gateway
COPY --from=go-build /out/proxy-gateway /usr/local/bin/
COPY deploy/test/init.yaml /config/init.yaml
COPY deploy/test/gateway.yaml /config/server.yaml
COPY deploy/test/gost.yaml /config/gost.yaml
ENTRYPOINT ["proxy-gateway", "-conf", "/config/init.yaml"]

FROM postgres:17 AS migrate
COPY --from=goose-build /go/bin/goose /usr/local/bin/
COPY apps/proxy-control/migrations /migrations
COPY deploy/test/seed.sql /seed.sql

FROM node:24-alpine AS console-build
WORKDIR /src
COPY apps/proxy-console/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY apps/proxy-console/ ./
RUN npm run build

FROM caddy:2-alpine AS console
COPY --from=console-build /src/dist /srv/console
COPY deploy/test/Caddyfile /etc/caddy/Caddyfile
