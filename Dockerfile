FROM golang:1.25-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bs-net-monitor main.go

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata iputils
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /build/bs-net-monitor /app/bs-net-monitor

EXPOSE 8701

ENTRYPOINT ["./bs-net-monitor"]
