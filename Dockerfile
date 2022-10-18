# syntax=docker/dockerfile:1.4
FROM golang:1.19.1-alpine3.16 AS go
RUN apk add --no-cache git gcc musl-dev
WORKDIR /go/metahub-api
COPY . ./
RUN --mount=type=cache,target=/go/pkg/mod go mod tidy
ENV CGO_ENABLED=1 GOOS=linux
RUN --mount=type=cache,target=/go/pkg/mod go build -a -ldflags '-extldflags "-static"' .

FROM golang:1.19.1-alpine3.16 AS dev
RUN apk add --no-cache git gcc musl-dev bash vim curl wget
WORKDIR /go/metahub-api
COPY . ./
RUN --mount=type=cache,target=/go/pkg/mod go mod tidy
ENV CGO_ENABLED=1 GOOS=linux
RUN --mount=type=cache,target=/go/pkg/mod go build -a -ldflags '-extldflags "-static"' .
COPY ./misc/docker/dev/bash_history /root/.bash_history
FROM alpine:3.16.2
ENV METAHUB_SERVER_PORT=80
COPY --from=go /go/metahub-api/metahub-api /usr/bin/metahub-api
CMD ["metahub-api", "serve"]
