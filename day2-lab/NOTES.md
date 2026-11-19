# Day 2 — Lab 2
## Memory Leaks & DB Profiling

### Goal

Learn how to:

- Find Node.js memory leaks using V8 Heap Snapshots.
- Identify closure and event-listener leaks.
- Profile slow PostgreSQL queries.
- Use `EXPLAIN ANALYZE`.
- Optimize queries using indexes.
- Understand and tune PostgreSQL connection pools.

---

# Part 1 — Memory Leaks

## 1. Closure Memory Leak

### Problem

A closure can keep an object alive even after the request has finished.

Example:

```js
const handlers = [];

app.get('/leak/closure', (req, res) => {
  const data = new ReportData(req.query.id);

  handlers.push(() => data.id);

  res.send('ok');
});
```

The important part is:

```js
handlers.push(() => data.id);
```

The function captures `data`.

Because the function is stored inside the global `handlers` array, the captured `data` cannot be garbage collected.

### Retainer Chain

```text
handlers[]
    ↓
function
    ↓
closure
    ↓
data
    ↓
ReportData
    ↓
rows[]
```

As requests continue, more objects remain in memory.

### How to detect

Use Chrome DevTools:

```text
node --inspect server.js
```

Open:

```text
chrome://inspect
```

Then:

```text
Memory → Heap Snapshot
```

Take snapshots before and after generating traffic.

Compare snapshots and look for objects that continue to grow.

---

# 2. Event Listener Memory Leak

### Problem

Adding a new listener for every request without removing it can retain request-specific objects.

Bad:

```js
app.get('/leak/listener', (req, res) => {
  const ctx = new RequestContext(req.query.id);

  bus.on('config-changed', () => ctx.refresh());

  res.send('ok');
});
```

Every request creates another listener.

### Retainer Chain

```text
EventEmitter
    ↓
listener
    ↓
closure
    ↓
ctx
    ↓
RequestContext
    ↓
buffer
```

### Fix

Store the listener reference and remove it when the response finishes:

```js
const onChange = () => ctx.refresh();

bus.on('config-changed', onChange);

res.on('finish', () => {
  bus.off('config-changed', onChange);
});
```

Now the listener does not remain attached after the request.

### Important

Do NOT use:

```js
bus.setMaxListeners(0);
```

as a memory-leak fix.

That only removes the warning. It does not remove the leaked listeners.

---

# Part 2 — PostgreSQL DB Profiling

## 3. Find a Slow Query

Example query:

```sql
SELECT id, amount, created_at
FROM orders
WHERE customer_id = $1
  AND status = $2
ORDER BY created_at DESC
LIMIT 20;
```

Before optimization, PostgreSQL performed a large scan.

Example:

```text
Parallel Seq Scan on orders
Rows Removed by Filter: 666664
Execution Time: 38.246 ms
```

The important problem:

```text
Seq Scan
```

PostgreSQL was scanning a large portion of the table to find matching rows.

---

# 4. Use EXPLAIN ANALYZE

Run:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, amount, created_at
FROM orders
WHERE customer_id = 12345
  AND status = 'paid'
ORDER BY created_at DESC
LIMIT 20;
```

Important things to inspect:

- Scan type
- Rows scanned
- Rows returned
- Rows removed by filter
- Execution time
- Buffer usage
- Sort operations

---

# 5. Add a Composite Index

Create:

```sql
CREATE INDEX CONCURRENTLY idx_orders_cust_status_created
ON orders (customer_id, status, created_at DESC);
```

Why these columns?

The query filters by:

```text
customer_id
status
```

and sorts by:

```text
created_at DESC
```

So the index matches the query pattern.

---

# 6. Before vs After

### Before

```text
Parallel Seq Scan
Execution Time: 38.246 ms
```

### After

```text
Bitmap Index Scan
Execution Time: 0.425 ms
```

Approximate improvement:

```text
38.246 ms → 0.425 ms
```

Around **90× faster** for this test query.

The important lesson:

> Don't blindly add indexes. First profile the query, understand the access pattern, then create an index that matches the workload.

---

# Part 3 — PostgreSQL Connection Pool

## 7. Configure the Pool

```js
const pool = new Pool({
  connectionString: 'postgres://postgres:dev@localhost:5432/postgres',
  max: Number(process.env.POOL_MAX || 10),
});
```

`max` controls the maximum number of database connections that this Node.js process can use.

---

# 8. Monitor Pool Status

```js
app.get('/pool', (req, res) => {
  res.json({
    total: pool.totalCount,
    idle: pool.idleCount,
    waiting: pool.waitingCount,
  });
});
```

### Meaning

```text
total
```

Total connections currently created by the pool.

```text
idle
```

Connections currently available.

```text
waiting
```

Requests waiting for a database connection.

---

# 9. Pool Load Test

The `/slow` endpoint intentionally holds a database connection:

```js
await pool.query('SELECT pg_sleep(0.05)');
```

With:

```text
POOL_MAX=10
```

and 50 concurrent requests:

```json
{
  "total": 10,
  "idle": 0,
  "waiting": 39
}
```

Meaning:

```text
10 connections → busy
39 requests → waiting
```

Increasing the pool:

```text
POOL_MAX=20
```

gave:

```json
{
  "total": 20,
  "idle": 0,
  "waiting": 30
}
```

After the workload finished:

```json
{
  "total": 20,
  "idle": 20,
  "waiting": 0
}
```

All 20 connections were idle and available.

---

# 10. How to Tune the Pool

Pool tuning does NOT mean:

```text
max = 100
```

or:

```text
max = 500
```

just because the server can handle it.

Choose the pool size based on actual workload.

Monitor:

```text
waiting connections
database CPU
database RAM
active DB connections
query latency
application latency
throughput
```

### General rule

If:

```text
waiting is consistently high
```

the pool may be too small **or** queries may be holding connections too long.

If:

```text
idle connections are consistently very high
```

the pool may be larger than necessary.

So:

```text
Load Test
    ↓
Observe Pool
    ↓
Observe DB CPU / RAM / Connections
    ↓
Check Latency
    ↓
Adjust pool size
    ↓
Test again
```

Never tune the pool using only one metric.

---

# Important Lessons

### Memory

```text
Memory leak
    ↓
Object remains reachable
    ↓
Garbage Collector cannot remove it
    ↓
Heap keeps growing
```

### Database

```text
Slow Query
    ↓
EXPLAIN ANALYZE
    ↓
Find expensive operation
    ↓
Index / Query optimization
    ↓
Measure again
```

### Connection Pool

```text
Incoming Requests
        ↓
Connection Pool
        ↓
Available DB Connection
        ↓
PostgreSQL
```

If all connections are busy:

```text
Request → waiting queue
```

---

# Useful Commands

## Start PostgreSQL

```bash
docker start pg
```

## Check PostgreSQL

```bash
docker exec pg pg_isready
```

## Start Node Server

```bash
node server.js
```

## Start with custom pool size — Windows CMD

```cmd
set POOL_MAX=20 && node server.js
```

## Check Docker container

```bash
docker ps
```

---

# Final Lab Result

Completed:

- [x] V8 Heap Snapshot
- [x] Closure memory leak
- [x] Event-listener memory leak
- [x] Event listener cleanup
- [x] PostgreSQL slow query profiling
- [x] EXPLAIN ANALYZE
- [x] Composite index optimization
- [x] Connection pool monitoring
- [x] Pool load testing
- [x] Pool tuning concept

## Key Takeaway

Production debugging is mainly:

```text
Observe
   ↓
Measure
   ↓
Find the bottleneck
   ↓
Change one thing
   ↓
Measure again
```

Don't optimize based on assumptions.
Use actual metrics and profiling data.