FROM golang:1.18-alpine3.16 AS build

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /root

COPY . /root

RUN go mod tidy && \
    CGO_ENABLED=0 go build -o process ./process.go && \
    chmod 777 process

FROM alpine:3.19

COPY --from=build /root/process /app/process

WORKDIR /app

ENTRYPOINT ["/app/process"]
