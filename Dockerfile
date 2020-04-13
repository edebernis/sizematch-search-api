FROM golang:1.13 as builder

WORKDIR /go/src/app
COPY . .

RUN GOOS=linux \
    CGO_ENABLED=0 \
    go build -a -installsuffix cgo -o sizematch-search-api

# ---------------------------------------------------------------------
FROM alpine:3.11
COPY --from=builder /go/src/app/sizematch-search-api /

ENTRYPOINT ["/sizematch-search-api"]
