FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go vet ./... && go test ./... && CGO_ENABLED=0 go build -o /circuit .

FROM alpine:3.20
COPY --from=build /circuit /circuit
EXPOSE 8080
ENTRYPOINT ["/circuit"]
