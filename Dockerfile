FROM golang:1.25.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o bridge main.go

FROM node:20-alpine AS plans-builder

WORKDIR /app

COPY static/plans/package.json static/plans/package-lock.json ./static/plans/
RUN cd static/plans && npm ci

COPY static/plans/ ./static/plans/
RUN cd static/plans && npm run build

FROM alpine:latest

RUN apk add --no-cache bash postgresql-client

WORKDIR /app

COPY --from=builder /app/bridge /bin/bridge
COPY --from=builder /app/ddl ./ddl
COPY --from=plans-builder /app/static/plans/dist ./static/plans/dist

EXPOSE 1323

ARG GIT_SHA
ENV GIT_SHA=$GIT_SHA
ENV GODEBUG=http2client=0

CMD ["bridge"] 
