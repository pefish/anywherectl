FROM golang:1.14
WORKDIR /app
ENV GO111MODULE=on
COPY ./ ./
RUN GOMAXPROCS=4 go test -timeout 90s -race ./...
RUN make
CMD ["./build/bin/anywherectl", "--help"]

# docker build -t pefish/anywherectl:v1.1 .