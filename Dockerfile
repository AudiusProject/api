FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bridge-amd64 main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bridge-amd64 /bin/bridge

EXPOSE 1323

CMD ["bridge"] 
