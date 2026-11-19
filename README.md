# Learning Tracker

This repo tracks my hands-on learning tasks. Each folder holds one task, with its own code and run steps.

**Author:** Mathan Raja

## Progress

| # | Task | Topics | Status |
|---|------|--------|--------|
| 1 | [day1-lab](day1-lab/) | Express, async error handling, event loop blocking, worker threads, VS Code debugging | ✅ Completed |
| 2 | [node-calculator-docker-basic-p](node-calculator-docker-basic-p/) | Node `http` module, REST API, input validation, static UI, Docker | ✅ Completed |
| 3 | [day2-lab](day2-lab/) | Heap snapshots, closure and event-listener leaks, `EXPLAIN ANALYZE`, composite indexes, PostgreSQL connection pool tuning | ✅ Completed |
| 4 | [day3-lab](day3-lab/) | AI-assisted OpenAPI spec, Zod validation, Jest + supertest tests, manual injection/auth audit | ✅ Completed |

---

## 1. day1-lab: Node.js async errors and event loop

An Express API built to reproduce and fix common Node.js production bugs.

### What it covers

| Route | Purpose |
|-------|---------|
| `GET /users/:id` | **Bug 1:** an `async` handler with no error handling. `/users/0` rejects, and Express 4 does not catch it, so the result is an unhandled promise rejection. Kept unfixed on purpose to show the problem. |
| `GET /crash/:id` | **Bug 2 fix:** the same handler wrapped in an `asyncHandler` helper that sends rejections to `next()`, so the global error handler returns a clean `500` JSON response. |
| `GET /report` | **Bug 3 fix:** a 2-second CPU-heavy task. The blocking version (kept commented out) froze the whole server; the fix moves the work into a worker thread (`report-worker.js`). |
| `GET /ping` | Health check. Use it to confirm the server stays responsive while `/report` runs. |
| `GET /event-loop` | Reports event-loop delay (max and mean, in ms) from `perf_hooks.monitorEventLoopDelay`, to measure blocking. |

### Run it

```bash
cd day1-lab
npm install
node server.js        # API on http://localhost:3000
```

To debug, open `day1-lab` in VS Code and use the **Debug API** launch configuration.

### Try it

```bash
curl http://localhost:3000/crash/0      # handled error → 500 {"error":"User not found"}
curl http://localhost:3000/report &     # runs in a worker thread
curl http://localhost:3000/ping         # still answers right away
curl http://localhost:3000/event-loop   # delay stays low
```

### What I learned

- Express 4 does not catch errors thrown in `async` route handlers; they need a wrapper (or Express 5).
- CPU-bound work on the main thread blocks every request; worker threads keep the event loop free.
- Event-loop delay is a direct, measurable signal of blocking.

---

## 2. node-calculator-docker-basic-p: calculator API in Docker

A calculator web app and JSON API built on Node's core `http` module (no dependencies), packaged with Docker.

### Endpoints

| Route | Description |
|-------|-------------|
| `GET /` | Calculator UI (`public/index.html`) |
| `GET /api` | Lists available endpoints |
| `GET /add?a=10&b=20` | Addition |
| `GET /subtract?a=10&b=20` | Subtraction |
| `GET /multiply?a=10&b=20` | Multiplication |
| `GET /divide?a=10&b=20` | Division |

Validation: missing or non-numeric `a`/`b` returns `400`, divide by zero returns `400`, and unknown routes return `404`.

Example response:

```json
{ "operation": "addition", "a": 10, "b": 20, "result": 30 }
```

### Run locally

```bash
cd node-calculator-docker-basic-p
npm start             # http://localhost:8000 (override with PORT)
```

### Run with Docker

```bash
cd node-calculator-docker-basic-p
docker build -t node-calculator .
docker run -p 8000:8000 node-calculator
```

Then open http://localhost:8000.

### What I learned

- Building an HTTP server and router with only Node core modules.
- Validating query input and returning proper HTTP status codes.
- Writing a Dockerfile, building an image, and mapping container ports.

---

## 3. day2-lab: memory leaks and PostgreSQL profiling

An Express + PostgreSQL API built to reproduce memory leaks, find a slow query, and observe connection pool behaviour. Full write-up in [day2-lab/NOTES.md](day2-lab/NOTES.md).

### What it covers

| Route | Purpose |
|-------|---------|
| `GET /leak/closure?id=` | **Leak 1:** a closure stored in a global array keeps each `ReportData` (and its 10,000-item array) alive. |
| `GET /leak/listener?id=` | **Leak 2:** a listener added to an `EventEmitter` on every request retains its `RequestContext`. Fixed by calling `bus.off` on `res` `finish`. |
| `GET /orders?customer=&status=` | Slow query on a 1M-row `orders` table. Logs `SLOW QUERY` when it takes over 50 ms. |
| `GET /slow` | Holds a pool connection for 50 ms (`pg_sleep`), used for pool load tests. |
| `GET /pool` | Reports pool `total`, `idle` and `waiting` counts. |
| `GET /debug` | Shows the number of `config-changed` listeners, to confirm the listener leak is fixed. |

### Run it

```bash
cd day2-lab
npm install
docker start pg                      # PostgreSQL container (password: dev)
psql -f seed.sql                     # create and fill the orders table (1M rows)
node server.js                       # API on http://localhost:3000
```

Heap snapshots: run `node --inspect server.js` and open `chrome://inspect`. Set the pool size with `POOL_MAX` (for example `set POOL_MAX=20 && node server.js` on Windows CMD).

### Results

| Check | Before | After |
|-------|--------|-------|
| `/orders` query | Parallel Seq Scan, 38.246 ms | Bitmap Index Scan, 0.425 ms (about 90× faster) |
| Pool, 50 concurrent `/slow` requests | `POOL_MAX=10`: 10 busy, 39 waiting | `POOL_MAX=20`: 20 busy, 30 waiting |

The fix for the slow query is a composite index that matches the filter and sort:

```sql
CREATE INDEX CONCURRENTLY idx_orders_cust_status_created
ON orders (customer_id, status, created_at DESC);
```

### What I learned

- A memory leak is an object that stays reachable. Heap snapshots show the retainer chain that keeps it alive.
- Listeners and closures held by long-lived objects must be removed when the request ends.
- Profile first with `EXPLAIN ANALYZE`, then add an index that matches the query's filter and sort.
- Do not set the pool size to a big number. Load test, then watch `waiting`, DB CPU and RAM, and latency together.

---

## 4. day3-lab: AI-assisted API and security auditing

An Express + PostgreSQL API with JWT auth. AI generated the OpenAPI spec, Zod validation and Jest tests, and I audited the output by hand for injection and auth flaws.

### What it covers

| Route | Purpose |
|-------|---------|
| `POST /users` | Register. Zod-style body check (rejects unknown keys), bcrypt hash, `409` on duplicate email. |
| `POST /login` | Returns a 1-hour JWT. Same `401` for unknown user and wrong password. |
| `GET /orders` | Current user's orders, filterable by status. Requires a token. |
| `GET /orders/:id` | One order, scoped to the owner. |
| `DELETE /admin/orders/:id` | Admin only. |

Docs: `openapi.yaml` (rendered as `redoc-static.html`).

### Run it

```bash
cd day3-lab
npm install
set JWT_SECRET=change-me && node server.js
npm test              # Jest + supertest, coverage in coverage/
```

### What I learned

- AI output is a draft: check every query is parameterised and every route has the right auth and ownership check.
- Validate input at the edge (reject unknown keys, check types) and keep error messages generic.
- A spec plus tests makes the audit repeatable.

---

## Repo structure

```
.
├── day1-lab/
│   ├── .vscode/launch.json
│   ├── report-worker.js
│   ├── server.js
│   └── package.json
├── node-calculator-docker-basic-p/
│   ├── public/index.html
│   ├── Dockerfile
│   ├── server.js
│   └── package.json
├── day2-lab/
│   ├── NOTES.md
│   ├── seed.sql
│   ├── server.js
│   └── package.json
├── day3-lab/
│   ├── openapi.yaml
│   ├── redoc-static.html
│   ├── server.js
│   ├── server.test.js
│   └── package.json
└── README.md
```

## Prerequisites

- Node.js 18 or later (the Docker image uses Node 24)
- Docker (for tasks 2 and 3; task 3 runs PostgreSQL in a container)
