# syntax=docker/dockerfile:1.4
FROM mirror.gcr.io/library/golang:1.26-alpine AS go
RUN apk add --no-cache gcc git musl-dev openssh-client
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan gitlab.com >> ~/.ssh/known_hosts
WORKDIR /go/metahub-api
COPY . ./
RUN mkdir -p ext
RUN --mount=type=ssh git clone git@github.com:MetaHub-Registry/metahub-core.git ext/metahub-core
RUN ls -la ext
RUN go mod edit \
     -replace github.com/metahub-registry/metahub-core=/go/metahub-api/ext/metahub-core \
     -replace github.com/metahub-registry/metahub-core/pkg/backends/minio=\
/go/metahub-api/ext/metahub-core/pkg/backends/minio \
     -replace github.com/metahub-registry/metahub-core/pkg/types=\
/go/metahub-api/ext/metahub-core/pkg/types \
     -replace github.com/metahub-registry/metahub-core/pkg/backends/sqlite=\
/go/metahub-api/ext/metahub-core/pkg/backends/sqlite \
     -replace github.com/metahub-registry/metahub-core/pkg/backends/neo=\
/go/metahub-api/ext/metahub-core/pkg/backends/neo

RUN --mount=type=cache,target=/go/pkg/mod go mod tidy
ENV CGO_ENABLED=1 GOOS=linux
RUN --mount=type=cache,target=/go/pkg/mod go build -a -ldflags '-extldflags "-static"' .

FROM mirror.gcr.io/library/alpine:3.21 AS gonja
RUN apk add --no-cache wget \
 && wget -qO- https://gitlab.com/qnib-golang/gonja-cli/-/releases/v0.0.2/downloads/gonja-cli_0.0.2_linux_amd64.tar.gz |tar xfz - -C /usr/local/bin


FROM mirror.gcr.io/library/golang:1.26-alpine  AS dev
RUN apk add --no-cache bash curl gcc git musl-dev openssh-client vim wget supervisor
COPY ./misc/docker/dev/supervisor.conf /etc/supervisord.conf
COPY ./misc/docker/dev/start.sh /usr/local/bin/start.sh
RUN chmod +x /usr/local/bin/start.sh
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan gitlab.com >> ~/.ssh/known_hosts
WORKDIR /go/metahub-api
# COPY . ./
# RUN mkdir -p ext
# RUN --mount=type=ssh git clone git@github.com:MetaHub-Registry/metahub-core.git ext/metahub-core
# RUN ls -la ext
# RUN go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/ext/metahub-core \
#  && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/minio=/go/metahub-api/ext/metahub-core/pkg/backends/minio \
#  && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/types=/go/metahub-api/ext/metahub-core/pkg/types \
#  && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/sqlite=/go/metahub-api/ext/metahub-core/pkg/backends/sqlite \
#  && go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/neo=/go/metahub-api/ext/metahub-core/pkg/backends/neo
# RUN go mod tidy
ENV CGO_ENABLED=1 GOOS=linux
ENV CFG_PATH=./misc/config/compose-config.yaml
COPY ./misc/docker/dev/bash_history /root/.bash_history
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf","-n"]

FROM mirror.gcr.io/library/alpine:3.21 AS prod
ARG USERDB_FILE=./metahub.db
ENV METAHUB_SERVER_PORT=80
COPY --from=go /go/metahub-api/metahub-api /usr/bin/metahub-api
COPY ${USERDB_FILE} /var/lib/metahub.db
COPY --from=go /go/metahub-api/metahub-api /usr/bin/metahub-api
RUN mkdir -p /var/metahub/run/
CMD ["/usr/local/bin/start.sh"]

FROM mirror.gcr.io/library/alpine:3.21
ARG USERDB_FILE=./metahub.db
ENV METAHUB_API_PORT=80 \
    METAHUB_DB=/var/lib/metahub/metahub.db \
    CFG_FILE=/etc/metahub/api.yaml \
    BLOB_BUCKET=blobs \
    MANIFESTS_BUCKET=manifests

COPY --from=gonja /usr/local/bin/gonja-cli /usr/local/bin/gonja-cli
RUN apk add --no-cache bash
COPY --from=go /go/metahub-api/metahub-api /usr/bin/metahub-api
COPY ./misc/config/metahub-api.yaml.tpl /opt/metahub-api/
COPY ${USERDB_FILE} /var/lib/metahub/metahub.db
VOLUME /var/lib/metahub
VOLUME /etc/metahub/
COPY ./misc/docker/bin/start.sh /usr/local/bin/start.sh
RUN mkdir -p /var/metahub/run/
CMD ["/usr/local/bin/start.sh"]