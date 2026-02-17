# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/nasne_exporter ./cmd/nasne_exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/nasne_exporter /nasne_exporter
EXPOSE 9900
ENTRYPOINT ["/nasne_exporter"]
