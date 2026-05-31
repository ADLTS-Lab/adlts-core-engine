FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-w -s" -o /app/adlts ./cmd/api/main.go


FROM alpine:3.20 AS runtime


RUN addgroup -S adlts && adduser -S adlts -G adlts

WORKDIR /app

COPY --from=builder /app/adlts .

RUN mkdir -p /uploads && chown adlts:adlts /uploads
VOLUME ["/uploads"]

USER adlts

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./adlts"]
