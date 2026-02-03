FROM golang:1.25.6-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o ipinfo .
RUN go build -ldflags="-s -w" -o healthcheck ./healthcheck/healthcheck.go

FROM alpine:latest
RUN apk add --no-cache tzdata

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /build/ipinfo .
COPY --from=builder /build/healthcheck .

RUN chown -R appuser:appgroup /app

USER appuser

ENV GEOIPUPDATE_EDITION_IDS="GeoLite2-City GeoLite2-ASN"

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["./healthcheck"]

EXPOSE 3000

CMD ["./ipinfo"]
