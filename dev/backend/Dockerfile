FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /keel-api ./cmd/api
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 keel
USER keel
WORKDIR /app
COPY --from=build /keel-api /app/keel-api
EXPOSE 8081
ENTRYPOINT ["/app/keel-api"]
