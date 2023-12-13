FROM golang:1.19.13-alpine3.18 AS builder

RUN apk add -U git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o deployer cmd/deployer/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o deployer-run cmd/runner/main.go

FROM alpine:3.18.3

COPY --from=builder /build/deployer /usr/bin/deployer
COPY --from=builder /build/deployer-run /deployer-run

ENTRYPOINT ["/usr/bin/deployer"]
