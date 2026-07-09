FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o loganalyze ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/loganalyze /usr/local/bin/
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["loganalyze"]
CMD ["serve", "--addr", ":8080", "--data", "/data"]
