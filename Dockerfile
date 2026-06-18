FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /bin/api ./cmd/api

FROM alpine:3.20

WORKDIR /app
COPY --from=build /bin/api /app/api
EXPOSE 8080
CMD ["/app/api"]
