FROM golang:1.26 AS builder

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/gobuild \
    CGO_ENABLED=0 GOOS=linux go build -v -ldflags="-s -w" -trimpath -o member-id .

FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /opt/member-id

COPY --from=builder /usr/src/app/member-id /opt/member-id/member-id

USER nonroot:nonroot

ENTRYPOINT ["/opt/member-id/member-id"]
