FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git

WORKDIR /app
COPY . .
RUN go get ./
RUN CGO_ENABLED=0 go build -o /app/http-mockery
RUN chmod +x /app/http-mockery

FROM alpine:latest

COPY --from=builder /app/http-mockery /go/bin/http-mockery
RUN apk update && apk add --no-cache ca-certificate
ENTRYPOINT ["/go/bin/http-mockery"]
