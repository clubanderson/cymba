FROM fedora:35 as builder

RUN dnf update -y
RUN dnf -y install go device-mapper-devel gcc gpgme-devel btrfs-progs-devel podman

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o cymba cmd/cymba.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
ENV USER_UID=10001

COPY --from=builder /workspace/cymba /

RUN microdnf update && microdnf clean all

USER ${USER_UID}