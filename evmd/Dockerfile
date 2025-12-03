FROM golang:1.25-alpine AS builder

RUN apk add --no-cache curl build-base git bash file linux-headers eudev-dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN LEDGER_ENABLED=false make build

FROM alpine:3.21
COPY --from=builder /src/build/evmd /bin/evmd
ENTRYPOINT ["evmd"]
