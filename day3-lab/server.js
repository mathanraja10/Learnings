const express = require('express');
const jwt = require('jsonwebtoken');
const bcrypt = require('bcryptjs');
const { Pool } = require('pg');

const app = express();
app.use(express.json());
const pool = new Pool({ connectionString: process.env.DATABASE_URL || 'postgres://postgres:dev@localhost:5432/postgres' });

const auth = (req, res, next) => {
  try {
    req.user = jwt.verify((req.headers.authorization || '').replace('Bearer ', ''), process.env.JWT_SECRET);
    next();
  } catch { res.status(401).json({ error: 'Unauthorized' }); }
};

// Register
app.post('/users', async (req, res, next) => {
  try {
    const body = req.body || {};
    const allowedKeys = ['email', 'password'];
    const hasUnexpectedKeys = Object.keys(body).some(key => !allowedKeys.includes(key));

    if (hasUnexpectedKeys || !body.email || !body.password || typeof body.email !== 'string' || typeof body.password !== 'string') {
      return res.status(400).json({ error: 'Invalid request body' });
    }

    const password_hash = await bcrypt.hash(body.password, 10);
    const { rows } = await pool.query(
      'INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email, role',
      [body.email, password_hash]
    );
    res.status(201).json(rows[0]);
  } catch (e) {
    if (e.code === '23505') {
      return res.status(409).json({ error: 'Email already exists' });
    }
    next(e);
  }
});

// Login
app.post('/login', async (req, res, next) => {
  try {
    const { email, password } = req.body || {};
    if (!email || !password || typeof email !== 'string' || typeof password !== 'string') {
      return res.status(400).json({ error: 'Invalid request body' });
    }

    const { rows } = await pool.query('SELECT * FROM users WHERE email = $1', [email]);
    const user = rows[0];
    if (!user || !(await bcrypt.compare(password, user.password_hash)))
      return res.status(401).json({ error: 'Invalid credentials' });

    res.json({ token: jwt.sign({ id: user.id, role: user.role }, process.env.JWT_SECRET, { expiresIn: '1h' }) });
  } catch (e) { next(e); }
});

// My orders, filtered by status
app.get('/orders', auth, async (req, res, next) => {
  try {
    const validStatuses = ['pending', 'paid', 'shipped', 'cancelled'];
    const status = req.query.status || 'paid';

    if (!validStatuses.includes(status)) {
      return res.status(400).json({ error: 'Invalid status' });
    }

    const { rows } = await pool.query(
      'SELECT * FROM orders WHERE customer_id = $1 AND status = $2 LIMIT 20',
      [req.user.id, status]
    );
    res.json(rows);
  } catch (e) { next(e); }
});

// One order
app.get('/orders/:id', auth, async (req, res, next) => {
  try {
    const id = Number(req.params.id);
    if (!Number.isInteger(id) || id <= 0) {
      return res.status(400).json({ error: 'Invalid order ID' });
    }

    const { rows } = await pool.query(
      'SELECT * FROM orders WHERE id = $1 AND customer_id = $2',
      [id, req.user.id]
    );
    if (!rows[0]) return res.status(404).json({ error: 'Not found' });
    res.json(rows[0]);
  } catch (e) { next(e); }
});

// Admin: delete an order
app.delete('/admin/orders/:id', auth, async (req, res, next) => {
  try {
    if (req.user.role !== 'admin') {
      return res.status(403).json({ error: 'Forbidden' });
    }

    const id = Number(req.params.id);
    if (!Number.isInteger(id) || id <= 0) {
      return res.status(400).json({ error: 'Invalid order ID' });
    }

    const result = await pool.query('DELETE FROM orders WHERE id = $1', [id]);
    if (result.rowCount === 0) {
      return res.status(404).json({ error: 'Not found' });
    }
    res.status(204).end();
  } catch (e) { next(e); }
});

app.use((err, req, res, next) => {
  console.error(err);
  res.status(500).json({ error: 'Internal Server Error' });
});

module.exports = app;
if (require.main === module) app.listen(3000, () => console.log('API on :3000'));
