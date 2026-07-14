FROM golang:1.23.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/controller ./cmd/controller && \
    CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/controller /app/controller
COPY --from=build /out/migrate /app/migrate
COPY db/migrations /app/db/migrations
WORKDIR /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/controller"]
