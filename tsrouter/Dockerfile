FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tsrouter .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /tsrouter /usr/bin/tsrouter
CMD ["/usr/bin/tsrouter"]
