# syntax=docker/dockerfile:1.4
FROM golang:1.19.1-alpine3.16 AS go
RUN apk add --no-cache git gcc musl-dev openssh-client
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan gitlab.com >> ~/.ssh/known_hosts
WORKDIR /go/metahub-api
COPY . ./
RUN mkdir -p ext
RUN --mount=type=ssh git clone git@github.com:MetaHub-Registry/metahub-core.git ext/metalib
RUN ls -la ext
RUN go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/ext/metalib \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/minio=/go/metahub-api/ext/metalib/pkg/backends/minio \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/types=/go/metahub-api/ext/metalib/pkg/types \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/sqlite=/go/metahub-api/ext/metalib/pkg/backends/sqlite \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/neo=/go/metahub-api/ext/metalib/pkg/backends/neo

RUN --mount=type=cache,target=/go/pkg/mod go mod tidy
ENV CGO_ENABLED=1 GOOS=linux
RUN --mount=type=cache,target=/go/pkg/mod go build -a -ldflags '-extldflags "-static"' .

FROM golang:1.19.1-alpine3.16 AS dev
RUN apk add --no-cache git gcc musl-dev bash vim curl wget openssh-client
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan gitlab.com >> ~/.ssh/known_hosts
WORKDIR /go/metahub-api
COPY . ./
RUN mkdir -p ext
RUN --mount=type=ssh git clone git@github.com:MetaHub-Registry/metahub-core.git ext/metalib
RUN ls -la ext
RUN go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/ext/metalib \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/minio=/go/metahub-api/ext/metalib/pkg/backends/minio \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/types=/go/metahub-api/ext/metalib/pkg/types \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/sqlite=/go/metahub-api/ext/metalib/pkg/backends/sqlite \
 && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/neo=/go/metahub-api/ext/metalib/pkg/backends/neo
RUN go mod tidy
ENV CGO_ENABLED=1 GOOS=linux
ENV CFG_PATH=./misc/config/compose-config.yaml
COPY ./misc/docker/dev/bash_history /root/.bash_history

FROM alpine:3.16.2 AS prod
ENV METAHUB_SERVER_PORT=80
COPY --from=go /go/metahub-api/metahub-api /usr/bin/metahub-api
COPY ./metahub.db /var/lib/metahub.db
COPY --from=go /go/metahub-api/metahub-api /usr/bin/metahub-api
RUN mkdir -p /var/metahub/run/
CMD ["/usr/local/bin/start.sh"]

FROM alpine:3.16.2
ENV METAHUB_SERVER_PORT=80 \
    CFG_FILE=/etc/metahub/api.yaml 
COPY ./misc/docker/bin/start.sh /usr/local/bin/start.sh
COPY --from=go /go/metahub-api/metahub-api /usr/bin/metahub-api
RUN mkdir -p /var/metahub/run/
CMD ["/usr/local/bin/start.sh"]