FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum .
RUN go mod download
COPY . .
RUN set -x && \
    go build -o /out/goapp

FROM golang:1.22
COPY --from=build /out/goapp /app/goapp
WORKDIR /app
CMD ["/app/goapp"]
