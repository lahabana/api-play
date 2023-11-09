FROM golang:1.21 as builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api-play


FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=builder \
    /api-play /usr/bin

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/usr/bin/api-play"]
