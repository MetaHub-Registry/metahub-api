FROM golang:1.18 AS mt-v1
WORKDIR /go/src/github.com/estesp/manifest-tool
RUN wget -qO - https://github.com/estesp/manifest-tool/releases/download/v1.0.3/manifest-tool-1.0.3.tar.gz|tar xfz - --strip-components=1 
RUN go mod init github.com/estesp/manifest-tool \
 && go mod tidy \
 && go mod vendor \
 && go build -o /usr/bin/manifest-tool
# && go mod edit -replace github.com/estesp/manifest-tool=github.com/estesp/manifest-tool@v1.0.3 \
# && go mod tidy \

FROM golang:1.18 AS mt-v2
ARG MT_VER=2.0.3
WORKDIR /go/src/github.com/estesp/manifest-tool
RUN wget -qO - https://github.com/estesp/manifest-tool/archive/refs/tags/v${MT_VER}.tar.gz|tar xfz - --strip-components=1 
WORKDIR /go/src/github.com/estesp/manifest-tool/v2
RUN go mod tidy \
 && cd cmd/manifest-tool \
 && go build -o /usr/bin/manifest-tool

FROM golang:1.18 AS regclient
ARG RC_VER=0.4.3
WORKDIR /go/src/github.com/regclient/regclient
RUN wget -qO - https://github.com/regclient/regclient/archive/refs/tags/v${RC_VER}.tar.gz|tar xfz - --strip-components=1 \
 && go mod tidy 
WORKDIR /go/src/github.com/regclient/regclient/cmd/regctl
RUN go build -o /usr/bin/regctl
WORKDIR /go/src/github.com/regclient/regclient/cmd/regbot
RUN go build -o /usr/bin/regbot
WORKDIR /go/src/github.com/regclient/regclient/cmd/regsync
RUN go build -o /usr/bin/regsync


FROM docker:20.10.14-dind
ENV MH_URL=dev:80
RUN apk add --no-cache bash curl jq screen skopeo vim
RUN wget -qO- https://github.com/oras-project/oras/releases/download/v1.3.0/oras_1.3.0_linux_arm64.tar.gz | tar xfz - -C /usr/local/bin \
  && chmod +x /usr/local/bin/oras
COPY daemon.json /etc/docker/daemon.json
COPY bash_history /root/.bash_history
COPY bashrc /root/.bashrc
COPY --from=mt-v1 /usr/bin/manifest-tool /usr/bin/manifest-tool
COPY --from=mt-v2 /usr/bin/manifest-tool /usr/bin/manifest-tool-v2
COPY --from=regclient /usr/bin/regctl /usr/bin/regctl
COPY --from=regclient /usr/bin/regsync /usr/bin/regsync
COPY --from=regclient /usr/bin/regbot /usr/bin/regbot