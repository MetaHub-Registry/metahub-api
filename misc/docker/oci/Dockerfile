FROM alpine:3.22

RUN apk --no-cache add bash ca-certificates git go
WORKDIR /usr/local/src
RUN git clone -b v1.0.1 https://github.com/opencontainers/distribution-spec.git
WORKDIR /usr/local/src/distribution-spec/conformance
RUN go test -c
