FROM golang:1.20
WORKDIR /app
COPY go.mod go.sum /app/
RUN CGO_ENABLED=0 go mod download
COPY . /app/
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -o eznoprimes

FROM gcr.io/distroless/base-debian10
COPY --from=0 /app/eznoprimes /app/eznoprimes
CMD ["/app/eznoprimes"]
