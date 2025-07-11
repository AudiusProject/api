FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bridge-amd64 main.go

FROM alpine:latest

RUN apk add --no-cache bash postgresql-client

WORKDIR /app

COPY --from=builder /app/bridge-amd64 /bin/bridge
COPY --from=builder /app/ddl ./ddl

EXPOSE 1323

CMD ["bridge"] 
