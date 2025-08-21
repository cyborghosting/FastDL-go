FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o fastdl ./cmd/fastdl


FROM busybox:stable

WORKDIR /app

COPY --from=builder /app/fastdl /app/fastdl

EXPOSE 8080

ENV GIN_MODE=release
ENV FASTDL_CONFIG=configuration.json

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:$PORT/health || exit 1

CMD [ "/app/fastdl" ]
