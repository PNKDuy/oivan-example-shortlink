# ---- build stage ----
# CGO is not required: modernc.org/sqlite is a pure-Go driver.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/shortlink ./cmd/server

# ---- run stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/shortlink /app/shortlink
ENV PORT=8080 DB_PATH=/data/shortlink.db
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/shortlink"]
