FROM golang:1.16 as builder
WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

ENV REMOTETAGS="remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp"
RUN GOOS=linux GOARCH=amd64 go build -tags "$REMOTETAGS" -o cymba cmd/cymba.go

FROM gcr.io/distroless/static:nonroot
ENV USER_UID=10001
WORKDIR /
COPY --from=builder /workspace/cymba /
COPY deploy/ deploy/

USER ${USER_UID}

ENTRYPOINT ["/cymba"]