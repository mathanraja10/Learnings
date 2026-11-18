# Learning Tracker

This repo tracks my hands-on learning tasks. Each folder holds one task, with its own code and run steps.

**Author:** Mathan Raja

## Progress

| # | Task | Topics | Status |
|---|------|--------|--------|
| 1 | [day1-lab](day1-lab/) | Express, async error handling, event loop blocking, worker threads, VS Code debugging | ✅ Completed |
| 2 | [node-calculator-docker-basic-p](node-calculator-docker-basic-p/) | Node `http` module, REST API, input validation, static UI, Docker | ✅ Completed |

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
└── README.md
```

## Prerequisites

- Node.js 18 or later (the Docker image uses Node 24)
- Docker (for task 2)
