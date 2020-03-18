FROM golang:1.14

WORKDIR /go/src

ARG GOLANGCILINT_VERSION=v1.24.0
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCILINT_VERSION}

COPY . .
