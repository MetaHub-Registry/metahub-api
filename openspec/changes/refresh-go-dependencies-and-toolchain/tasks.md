# Tasks — Go 1.26 dependency refresh

Status: **complete** in the `open-source-metahub` session (2026-10-27). Kept as
the durable record of the policy.

- [x] 1.1 Raise the `go` directive and align the builder image (`golang:1.26`).
- [x] 1.2 Refresh the graph in one pass (`go get -u ./...` + `go mod tidy`),
      including `golang.org/x/crypto`, `golang.org/x/net`, and the
      `go.opentelemetry.io/otel` family.
- [x] 1.3 Fix the `go vet` format-string fallout from the raised floor.
- [x] 1.4 Regenerate `THIRD_PARTY_NOTICES.md` on linux/amd64.
- [x] 1.5 Re-pin the consumers after the branch was final.
- [x] 1.6 Dev + prod pipelines green; PR merged with DCO sign-off.
