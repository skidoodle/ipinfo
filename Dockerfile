FROM golang:1.25.6-alpine AS builder

RUN apk update && apk add --no-cache git ca-certificates tzdata

RUN addgroup -S -g 10001 appgroup && \
    adduser -S -u 10001 -G appgroup appuser

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ipinfo .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o healthcheck ./healthcheck/healthcheck.go

WORKDIR /staging
RUN cp /build/ipinfo . && \
    cp /build/healthcheck/healthcheck .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder --chown=10001:10001 /staging /app

WORKDIR /app

USER 10001

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["./healthcheck"]

EXPOSE 3000

CMD ["./ipinfo"]
