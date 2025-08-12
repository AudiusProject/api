FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o bridge main.go

FROM alpine:latest

RUN apk add --no-cache bash postgresql-client

WORKDIR /app

COPY --from=builder /app/bridge /bin/bridge
COPY --from=builder /app/ddl ./ddl

EXPOSE 1323

ARG GIT_SHA
ENV GIT_SHA=$GIT_SHA

CMD ["bridge"] 
