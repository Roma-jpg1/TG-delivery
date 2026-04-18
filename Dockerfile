FROM golang:1.26 AS builder
WORKDIR /src

COPY go.mod ./
COPY . ./

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/tg-delivery ./main.go

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/tg-delivery /app/tg-delivery

EXPOSE 8080
ENTRYPOINT ["/app/tg-delivery"]
