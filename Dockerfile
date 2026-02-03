FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go controller.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o packet-capture .

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    bash \
    tcpdump \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/packet-capture /usr/local/bin/packet-capture

RUN mkdir -p /captures

ENTRYPOINT ["packet-capture"]
