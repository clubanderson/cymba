# Build the manager binary
#FROM golang:1.17 as builder
FROM fedora:35 as builder

RUN dnf update -y
RUN dnf -y install go device-mapper-devel gcc gpgme-devel btrfs-progs-devel podman

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/
#COPY controllers/ controllers/

# Build
RUN GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager cmd/controllermanager/manager.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
# WORKDIR /
# COPY --from=builder /workspace/manager .
# USER 65532:65532

# ENTRYPOINT ["/manager"]

FROM fedora:35
RUN dnf update -y
RUN dnf -y install go device-mapper-devel gcc gpgme-devel btrfs-progs-devel podman

ENV USER_UID=10001

COPY --from=builder /workspace/manager /

USER ${USER_UID}