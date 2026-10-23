FROM golang:1.26 AS build

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/rate-limiter ./cmd

FROM scratch

WORKDIR /app
COPY --from=build /app/rate-limiter .
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080

ENTRYPOINT ["./rate-limiter"]