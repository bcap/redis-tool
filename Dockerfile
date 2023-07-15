FROM golang:alpine as build

WORKDIR /app

# download & build deps
COPY go.mod go.sum ./
RUN go mod download -x
RUN find /go/pkg/mod -name 'go.sum' | xargs -I {} sh -c 'cd $(dirname {}) && go build .'

# build 
COPY . .
RUN go build .

# dist
FROM alpine

WORKDIR /app

COPY --from=build /app/redis-tool .

ENTRYPOINT [ "/app/redis-tool" ]