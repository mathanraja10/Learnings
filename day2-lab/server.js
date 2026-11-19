const express = require('express');
const { EventEmitter } = require('events');
const { Pool } = require('pg');

const app = express();

const pool = new Pool({
  connectionString: 'postgres://postgres:dev@localhost:5432/postgres',
  max: Number(process.env.POOL_MAX || 10),
});

// Print heap usage every 5 seconds
// setInterval(() => {
//   console.log(
//     `heapUsed: ${(process.memoryUsage().heapUsed / 1024 / 1024).toFixed(1)} MB`
//   );
// }, 5000);
// ---------- LEAK 1: closure ----------
class ReportData {
  constructor(id) {
    this.id = id;
    this.rows = new Array(10000).fill(id);
  }
}

const handlers = [];

app.get('/leak/closure', (req, res) => {
  const data = new ReportData(req.query.id);

  handlers.push(() => data.id);

  res.send('ok');
});

// ---------- LEAK 2: event listener ----------
class RequestContext {
  constructor(id) {
    this.id = id;
    this.buffer = new Array(5000).fill(id);
  }

  refresh() {
    /* reload config */
  }
}

const bus = new EventEmitter();

app.get('/leak/listener', (req, res) => {
  const ctx = new RequestContext(req.query.id);

//   bus.on('config-changed', () => ctx.refresh());

  // Fixing code for memory leak by removing the event listener when the response is finished
  const onChange = () => ctx.refresh();
  bus.on('config-changed', onChange);

  res.on('finish', () => {
    bus.off('config-changed', onChange);
  });
    
  res.send('ok');
});

// ---------- SLOW QUERY ----------
app.get('/orders', async (req, res, next) => {
  try {
    const t = Date.now();

    const { rows } = await pool.query(
      `SELECT id, amount, created_at
       FROM orders
       WHERE customer_id = $1
         AND status = $2
       ORDER BY created_at DESC
       LIMIT 20`,
      [req.query.customer, req.query.status]
    );

    const ms = Date.now() - t;

    if (ms > 50) {
      console.warn(`SLOW QUERY ${ms} ms`);
    }

    res.json(rows);
  } catch (e) {
    next(e);
  }
});

// ---------- POOL TEST ----------
app.get('/slow', async (req, res, next) => {
  try {
    await pool.query('SELECT pg_sleep(0.05)');
    res.send('ok');
  } catch (e) {
    next(e);
  }
});

app.get('/pool', (req, res) => {
  res.json({
    total: pool.totalCount,
    idle: pool.idleCount,
    waiting: pool.waitingCount,
  });
});

app.get('/debug', (req, res) => {
  res.json({
    configChangedListeners: bus.listenerCount('config-changed'),
  });
});

app.listen(3000, () => {
  console.log('API on :3000');
});