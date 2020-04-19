FROM golang:1.14 AS builder
WORKDIR /app
ENV GO111MODULE=on
COPY ./ ./
RUN GOMAXPROCS=1 go test -timeout 90s ./...
RUN GOMAXPROCS=4 go test -timeout 90s -race ./...
RUN make


FROM scratch as final
COPY --from=builder /app/build/* /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV GO_CONFIG /config/pom.yaml
ENV GO_SECRET /secret/pom.yaml
# CMD ["./main"]

# docker build -t pefish/golang-app-template .