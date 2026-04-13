FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /relay cmd/server/main.go

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /relay /relay
EXPOSE 8080
CMD ["/relay"]
