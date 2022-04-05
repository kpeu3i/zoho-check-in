FROM golang:1.18 as build

WORKDIR /build

COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -o zpcheckin

FROM zenika/alpine-chrome:latest

COPY --from=build /build/zpcheckin /usr/local/bin/zpcheckin

ENTRYPOINT ["zpcheckin"]
