FROM golang:alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o ipinfo .
RUN go build -ldflags="-s -w" -o healthcheck ./healthcheck/healthcheck.go
RUN go install github.com/maxmind/geoipupdate/v7/cmd/geoipupdate@latest

FROM alpine:latest
RUN apk add --no-cache curl tzdata busybox-suid

WORKDIR /app
COPY --from=builder /build/ipinfo .
COPY --from=builder /build/healthcheck .
COPY --from=builder /go/bin/geoipupdate /usr/local/bin/geoipupdate

ENV GEOIPUPDATE_ACCOUNT_ID=${GEOIPUPDATE_ACCOUNT_ID}
ENV GEOIPUPDATE_LICENSE_KEY=${GEOIPUPDATE_LICENSE_KEY}
ENV GEOIPUPDATE_EDITION_IDS="GeoLite2-City GeoLite2-ASN"
ENV GEOIPUPDATE_DB_DIR=/app

RUN echo "AccountID ${GEOIPUPDATE_ACCOUNT_ID}" > /etc/GeoIP.conf && \
    echo "LicenseKey ${GEOIPUPDATE_LICENSE_KEY}" >> /etc/GeoIP.conf && \
    echo "EditionIDs ${GEOIPUPDATE_EDITION_IDS}" >> /etc/GeoIP.conf && \
    echo "DatabaseDirectory ${GEOIPUPDATE_DB_DIR}" >> /etc/GeoIP.conf

RUN echo "0 0 * * * geoipupdate >> /var/log/geoipupdate.log 2>&1" > /etc/crontabs/root
RUN cat /etc/crontabs/root

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["./healthcheck"]

EXPOSE 3000

CMD ["./ipinfo"]
