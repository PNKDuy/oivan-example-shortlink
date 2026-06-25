# ---- build stage ----
# CGO is not required: modernc.org/sqlite is a pure-Go driver.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/shortlink ./cmd/server
# Pre-create the data dir so it can be copied with nonroot ownership below;
# the distroless runtime user (uid 65532) must be able to write the SQLite file.
RUN mkdir -p /data

# ---- run stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/shortlink /app/shortlink
# /data is the SQLite volume mount point; own it as the nonroot user (65532)
# so the app can create shortlink.db. Without this, SQLite fails with
# "unable to open database file" (SQLITE_CANTOPEN).
COPY --from=build --chown=65532:65532 /data /data
ENV PORT=8080 DB_PATH=/data/shortlink.db
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/shortlink"]
