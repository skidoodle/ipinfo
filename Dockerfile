FROM golang:alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ipinfo .
RUN mkdir -p /build/data

FROM alpine:latest
WORKDIR /app
COPY --from=builder /build/ipinfo .
COPY --from=builder /build/data /app/data
EXPOSE 3000
CMD ["./ipinfo"]
