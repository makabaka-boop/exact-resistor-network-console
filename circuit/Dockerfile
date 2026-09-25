FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/circuit .

FROM alpine:3.21
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/circuit /usr/local/bin/circuit
EXPOSE 8080
ENTRYPOINT ["circuit"]
