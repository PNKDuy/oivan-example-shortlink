# ShortLink — a URL shortening service

ShortLink turns a long URL such as `https://codesubmit.io/library/react` into a
short one such as `http://localhost:8080/GeAi9K`, and resolves it back again.
Mappings are stored in SQLite, so **encoded URLs are still decodable after a
restart**.

- **Language:** Go 1.22+
- **HTTP framework:** [Gin](https://github.com/gin-gonic/gin)
- **Storage:** SQLite via the pure-Go [`modernc.org/sqlite`](https://modernc.org/sqlite) driver (no CGO)
- **Short code:** 7-character random base62

**🔗 Live demo:** http://14.225.211.201:8080

Copy-paste against the live server (no setup needed):

```bash
# Encode a URL -> {"code":"...","short_url":"http://14.225.211.201:8080/..."}
curl -s -X POST http://14.225.211.201:8080/encode \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://codesubmit.io/library/react"}'

# Decode by full short URL -> {"url":"https://codesubmit.io/library/react"}
curl -s -X POST http://14.225.211.201:8080/decode \
  -H 'Content-Type: application/json' \
  -d '{"short_url":"http://14.225.211.201:8080/sjLzyiC"}'

# Decode by bare code (also accepted)
curl -s -X POST http://14.225.211.201:8080/decode \
  -H 'Content-Type: application/json' \
  -d '{"short_url":"sjLzyiC"}'

# Follow the short link (302 redirect to the original URL)
curl -sIL http://14.225.211.201:8080/sjLzyiC | grep -i '^location'

# Health check -> {"status":"ok"}
curl -s http://14.225.211.201:8080/healthz
```

Encode then decode in one go (extracts the code with `jq`):

```bash
CODE=$(curl -s -X POST http://14.225.211.201:8080/encode \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/hello"}' | jq -r .code)

curl -s -X POST http://14.225.211.201:8080/decode \
  -H 'Content-Type: application/json' \
  -d "{\"short_url\":\"$CODE\"}"
```

Error cases (note the status codes):

```bash
# Dangerous scheme is rejected -> 400
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://14.225.211.201:8080/encode \
  -H 'Content-Type: application/json' -d '{"url":"javascript:alert(1)"}'

# Unknown code -> 404
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://14.225.211.201:8080/decode \
  -H 'Content-Type: application/json' -d '{"short_url":"zzzzzzz"}'
```

> Replace `http://14.225.211.201:8080` with `localhost:8080` to run the same
> commands against a local instance. The `sjLzyiC` code above is a real,
> pre-seeded mapping on the live demo.

---

## Quick start

No database to install — SQLite is an embedded file, and the driver is pure Go.

```bash
# 1. Run the server (creates ./shortlink.db on first run)
make run          # or: go run ./cmd/server

# 2. Encode a URL
curl -s -X POST localhost:8080/encode \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://codesubmit.io/library/react"}'
# -> {"code":"GeAi9K","short_url":"http://localhost:8080/GeAi9K"}

# 3. Decode it back
curl -s -X POST localhost:8080/decode \
  -H 'Content-Type: application/json' \
  -d '{"short_url":"http://localhost:8080/GeAi9K"}'
# -> {"url":"https://codesubmit.io/library/react"}

# 4. Or just open the short URL in a browser — it 302-redirects.
```

### With Docker

```bash
docker build -t shortlink .
docker run -d --name shortlink --restart unless-stopped \
  -p 8080:8080 -v shortlink-data:/data \
  -e DB_PATH=/data/shortlink.db \
  -e BASE_URL=http://localhost:8080 \
  shortlink
```

The image is a distroless static binary running as a non-root user. The
`shortlink-data` volume holds the SQLite file, so mappings survive
`docker restart` / redeploys. Set `BASE_URL` to the public address so the
`short_url` field in responses points at the right host.

### Deployment

The live demo above runs this exact image on a VPS:

```bash
git clone https://github.com/PNKDuy/oivan-example-shortlink.git
cd oivan-example-shortlink
docker build -t shortlink .
docker run -d --name shortlink --restart unless-stopped \
  -p 8080:8080 -v shortlink-data:/data \
  -e DB_PATH=/data/shortlink.db \
  -e BASE_URL=http://<PUBLIC_IP>:8080 \
  shortlink
```

Any host with Docker works (VPS, Fly.io, Render, etc.). Persistence is verified
end-to-end against the live instance: encode a URL, `docker restart shortlink`,
and the code still decodes (the volume outlives the container).

### Configuration

All optional; see [`.env.example`](.env.example). Set via environment variables:

| Variable      | Default                  | Description                                |
|---------------|--------------------------|--------------------------------------------|
| `PORT`        | `8080`                   | HTTP listen port                           |
| `DB_PATH`     | `shortlink.db`           | SQLite file path                           |
| `BASE_URL`    | `http://localhost:$PORT` | Base used to build short URLs in responses |
| `CODE_LENGTH` | `7`                      | Length of generated codes                  |
| `MAX_RETRIES` | `5`                      | Collision retries before failing           |

---

## API

All endpoints accept and return JSON.

### `POST /encode`

| Request | Response (200) |
|---------|----------------|
| `{"url": "https://example.com/x"}` | `{"code": "GeAi9K", "short_url": "http://localhost:8080/GeAi9K"}` |

- `400` — body is not valid JSON, `url` is missing/empty, the scheme is not
  `http`/`https`, the host is missing, or the URL exceeds 2048 characters.
- `500` — a unique code could not be generated within `MAX_RETRIES`, or storage failed.

### `POST /decode`

Accepts either a full short URL or a bare code, in either field:

| Request | Response (200) |
|---------|----------------|
| `{"short_url": "http://localhost:8080/GeAi9K"}` | `{"url": "https://example.com/x"}` |
| `{"short_url": "GeAi9K"}` | `{"url": "https://example.com/x"}` |

- `400` — body is not valid JSON or the code is malformed (not base62).
- `404` — the code is unknown.

### `GET /:code`

Convenience: `302` redirect to the original URL (so a short link works in a
browser). `404` if unknown.

### `GET /healthz`

Liveness probe → `200 {"status":"ok"}`.

---

## Design decisions

### Short code strategy: random base62

A code is 7 random characters from `[A-Za-z0-9]` generated with `crypto/rand`.
Uniqueness is enforced by the storage layer (the code is the primary key); on
the rare collision the service simply generates another code and retries.

I considered three strategies:

| Strategy | Pros | Cons | Verdict |
|---|---|---|---|
| **Random base62** (chosen) | Not enumerable; trivial to scale horizontally (stateless); leaks no business metrics | Needs a uniqueness check + retry on collision | ✅ Best balance for this scope |
| **Sequential counter → base62** | Never collides; shortest codes | **Enumerable** (`/0G` → guess `/0H`); leaks total link count; needs a shared/distributed counter to scale | ❌ Security & scaling cost |
| **Hash(URL) truncated** | Idempotent (dedupes URLs) | Collisions are inevitable and must be resolved; acts as an oracle (anyone can confirm a URL was shortened) | ❌ Complexity & privacy |

`crypto/rand` (not `math/rand`) is used specifically so codes can't be
predicted — see the threat model below.

### Storage: SQLite behind a `Repository` interface

Business logic depends only on a small interface:

```go
type Repository interface {
    Save(ctx context.Context, code, longURL string) error // ErrCodeExists on duplicate
    Get(ctx context.Context, code string) (string, error) // ErrNotFound if absent
    Close() error
}
```

Two implementations ship: `SQLite` (persistent, used to run) and `Memory`
(used in tests — fast, no I/O, race-safe). Swapping in MySQL or Postgres for
production means writing one more implementation and changing nothing above
this seam. A shared conformance test runs against both so they stay identical.

SQLite was chosen because it satisfies the persistence requirement with **zero
setup for the reviewer** — no server, no container, no migrations to run. The
pure-Go driver keeps `go build`/`go test` working everywhere without a C
toolchain, and the Docker image stays a tiny static binary. PRAGMAs
(`journal_mode=WAL`, `busy_timeout`) are set via the DSN so every pooled
connection inherits them; the pool is capped to one connection because SQLite
allows only a single writer (this is exercised by the concurrency test).

### Idempotency: off by default

Encoding the same URL twice yields two different codes. This keeps the write
path simple and matches the plain meaning of "shorten". To dedupe instead, add
a `UNIQUE` index on `long_url` and look it up before generating — at the cost of
an extra read per encode and the privacy oracle noted above.

---

## Persistence across restart

Mappings live in the SQLite file at `DB_PATH`. Restarting the process reopens
the same file and all codes remain decodable. This is proven by
`TestSQLite_PersistsAcrossRestart`, which writes a mapping, **closes** the
store (simulating shutdown), opens a **fresh** store against the same file, and
asserts the value is still there.

---

## Project structure

```
cmd/server/         # main: config, wiring, graceful shutdown
internal/
  shortener/        # business logic: validation, encode/decode, code generation
  repository/       # Repository interface + SQLite and in-memory implementations
  httpapi/          # Gin handlers, request/response shapes, error→status mapping
```

The layering is one-directional: `httpapi` → `shortener` → `repository`. The
domain never imports the framework or the driver.

---

## Testing

```bash
make test        # go test ./...
make test-race   # go test ./... -race   (catches data races)
```

Coverage spans unit and HTTP-level tests:

- **Endpoints** (`httpapi`): encode/decode round trip, malformed JSON, invalid
  URLs, dangerous schemes, unknown code (404), redirect, health.
- **Service** (`shortener`): URL validation table (empty, no scheme,
  `javascript:`/`data:`/`file:`, missing host, over-length), **collision retry**,
  **retry exhaustion**, decode by bare code *and* full URL, malformed code.
- **Repository**: a shared conformance suite for both implementations;
  **restart persistence**; and **concurrent writes** verified under `-race`,
  asserting no code is ever assigned two URLs.
- **Generator**: length, charset, and a large-sample uniqueness sanity check.

---

## Security — threat model

The service stores and returns user-supplied URLs, which is the root of most of
its risk. Identified vectors and how this implementation handles them:

| Threat | Risk | Status here |
|---|---|---|
| **Malicious scheme injection** | Shortening `javascript:`, `data:`, `file:` URLs turns the service into an obfuscation tool for XSS / local-file payloads | **Mitigated** — only `http`/`https` schemes are accepted; everything else is `400`. |
| **Code enumeration / scraping** | Guessing codes to harvest everyone's links | **Mitigated** — codes are `crypto/rand` base62, not sequential, so they can't be enumerated. (At scale, add rate limiting — see below.) |
| **Open redirect / phishing** | The redirect endpoint can send users to attacker URLs; short links hide the destination | **Documented / partial** — inherent to any shortener. The `/decode` API returns the URL as data without redirecting; only `GET /:code` redirects. Production hardening: domain blocklist / Google Safe Browsing, and an interstitial warning page. |
| **SSRF** | A URL pointing at internal addresses (`169.254.169.254`, `localhost`) | **Not applicable by design** — the service never *fetches* the URL, it only stores the string. Any future link-preview feature must block private/link-local IP ranges. |
| **SQL injection** | User input reaching the database | **Mitigated** — all queries are parameterized; no string concatenation. |
| **Reflected XSS** | A URL containing markup rendered by a client | **Mitigated** — responses are JSON with `Content-Type: application/json`; the API never renders HTML. |
| **Resource exhaustion / abuse** | Spamming `/encode` to bloat storage; giant request bodies | **Partial** — URLs are capped at 2048 chars. Production: per-IP rate limiting, request body size limits, and auth/quotas. |
| **Code generation failure** | An attacker fills the keyspace to force collisions | **Bounded** — retries are capped (`MAX_RETRIES`) and the keyspace (62⁷) makes this infeasible at this scale; widen `CODE_LENGTH` if needed. |

---

## Scalability & the collision problem

> The brief asks to *think through* scaling and collisions, not to build a
> scalable service. This section describes the path; the demo intentionally
> stays simple.

### Collision problem

With 7-character base62 codes the keyspace is 62⁷ ≈ **3.5 × 10¹²**. By the
birthday bound, collision probability reaches ~50% only after roughly **2.2
million** stored links, so at small/medium scale random generation almost never
collides. When it does, the primary-key constraint rejects the insert
(`ErrCodeExists`) and the service retries with a new code.

As write volume grows, retrying against the live table gets expensive (each
attempt is a DB round trip). The standard fix is a **Key Generation Service
(KGS)**: pre-generate unique codes offline into a pool, and have writers *claim*
an unused key rather than gamble at request time:

```sql
WITH candidate AS (
  SELECT * FROM keys
  WHERE claimed = false
  LIMIT 1
  FOR UPDATE SKIP LOCKED      -- skip rows another writer is claiming; never block
)
UPDATE keys SET claimed = true, long_url = $1 FROM candidate WHERE keys.id = candidate.id
RETURNING keys.code;
```

`FOR UPDATE SKIP LOCKED` removes the write contention that would otherwise occur
when many writers fight over the same rows. Recycling expired links (a
`expiry_time` column) keeps the key pool from being exhausted. This moves
collision handling out of the hot path entirely.

### Scaling the rest

Reference figures for a Bitly-scale system: ~228 writes/s, ~3,805 reads/s, and
~48 TB of mappings accumulated over a century — a heavily **read-skewed** (~17:1)
workload.

- **Caching** — reads dominate, and access is hot-skewed (a small fraction of
  links serve most traffic), so a look-aside cache (Redis) in front of the
  store absorbs the bulk of reads.
- **Database** — a single node handles the demo and even moderate production
  load; beyond that, **shard by code** (e.g. Citus/Vitess). Because codes are
  random they distribute evenly across shards with no hot partition.
- **Index trade-off** — a B-tree (PostgreSQL) suits the read-heavy lookup; an
  LSM-tree store favours write-heavy ingestion. Choose per workload.
- **Replication & regions** — single-leader replication with read replicas for
  read scaling; multi-region read replicas (and edge caching) for global
  latency.
- **App tier** — the service is stateless (all state is in the store), so it
  scales horizontally behind a load balancer with no coordination — a direct
  benefit of choosing random codes over a shared counter.

---

## What I'd add for production

Deliberately out of scope for this exercise, in rough priority order:

1. Per-IP rate limiting and request-size limits on `/encode`.
2. Authentication / API keys and per-user quotas.
3. Structured logging, metrics (`/metrics`), and tracing.
4. URL safety checks (Safe Browsing) and a redirect interstitial.
5. Custom aliases, link expiry, and click analytics.
6. Swap SQLite for Postgres + Redis (one new `Repository` implementation).

## Assumptions

- No authentication; the API is open (a demo).
- Encoding is not idempotent (same URL → new code each time).
- Single-node deployment; horizontal scaling is described, not implemented.
