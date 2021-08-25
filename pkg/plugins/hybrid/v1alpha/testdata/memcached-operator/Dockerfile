# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.4

ENV HOME=/opt/helm \
    USER_ID=65532

# Copy necessary files with the right permissions
COPY --chown=${USER_ID}:${USER_ID} watches.yaml ${HOME}/watches.yaml
COPY --chown=${USER_ID}:${USER_ID} helm-charts  ${HOME}/helm-charts

# Copy manager binary
COPY --from=builder /workspace/manager .
USER 65532:65532

WORKDIR ${HOME}

ENTRYPOINT ["/manager"]
