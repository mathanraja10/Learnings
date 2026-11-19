process.env.JWT_SECRET = 'test-jwt-secret-for-testing-purposes-123';
process.env.DATABASE_URL = 'postgres://dummy:dummy@localhost:5432/test';

const request = require('supertest');
const jwt = require('jsonwebtoken');
const bcrypt = require('bcryptjs');

const mockQuery = jest.fn();

jest.mock('pg', () => {
  return {
    Pool: jest.fn().mockImplementation(() => ({
      query: mockQuery,
    })),
  };
});

// Import app after mocking pg
const app = require('./server');

describe('Security & Functionality Test Suite', () => {
  let userToken;
  let adminToken;
  let passwordHash;

  beforeAll(async () => {
    passwordHash = await bcrypt.hash('CorrectPassword123', 10);
    userToken = jwt.sign({ id: 1, role: 'user' }, process.env.JWT_SECRET, { expiresIn: '1h' });
    adminToken = jwt.sign({ id: 99, role: 'admin' }, process.env.JWT_SECRET, { expiresIn: '1h' });
  });

  beforeEach(() => {
    mockQuery.mockReset();
    jest.spyOn(console, 'error').mockImplementation(() => { });
  });

  afterEach(() => {
    console.error.mockRestore();
  });

  describe('POST /users (Registration)', () => {
    it('successfully registers a user and returns id, email, role without password_hash', async () => {
      mockQuery.mockResolvedValueOnce({
        rows: [{ id: 1, email: 'newuser@example.com', role: 'user' }],
      });

      const res = await request(app)
        .post('/users')
        .send({ email: 'newuser@example.com', password: 'SecretPassword123' });

      expect(res.status).toBe(201);
      expect(res.body).toEqual({ id: 1, email: 'newuser@example.com', role: 'user' });
      expect(res.body.password_hash).toBeUndefined();
      expect(mockQuery).toHaveBeenCalledTimes(1);
      expect(mockQuery.mock.calls[0][0]).toContain('INSERT INTO users (email, password_hash) VALUES ($1, $2)');
      expect(mockQuery.mock.calls[0][1][0]).toBe('newuser@example.com');
    });

    it('rejects unexpected fields (e.g. role) with 400 Bad Request to prevent mass assignment', async () => {
      const res = await request(app)
        .post('/users')
        .send({ email: 'attacker@example.com', password: 'password123', role: 'admin' });

      expect(res.status).toBe(400);
      expect(res.body.error).toBe('Invalid request body');
      expect(mockQuery).not.toHaveBeenCalled();
    });

    it('rejects request with missing password with 400 Bad Request', async () => {
      const res = await request(app)
        .post('/users')
        .send({ email: 'user@example.com' });

      expect(res.status).toBe(400);
      expect(res.body.error).toBe('Invalid request body');
      expect(mockQuery).not.toHaveBeenCalled();
    });

    it('returns 409 Conflict when email already exists (code 23505)', async () => {
      const uniqueError = new Error('duplicate key');
      uniqueError.code = '23505';
      mockQuery.mockRejectedValueOnce(uniqueError);

      const res = await request(app)
        .post('/users')
        .send({ email: 'existing@example.com', password: 'Password123' });

      expect(res.status).toBe(409);
      expect(res.body.error).toBe('Email already exists');
    });
  });

  describe('POST /login (Authentication)', () => {
    it('returns a JWT with expiration on valid credentials', async () => {
      mockQuery.mockResolvedValueOnce({
        rows: [{ id: 1, email: 'user@example.com', password_hash: passwordHash, role: 'user' }],
      });

      const res = await request(app)
        .post('/login')
        .send({ email: 'user@example.com', password: 'CorrectPassword123' });

      expect(res.status).toBe(200);
      expect(res.body).toHaveProperty('token');

      const decoded = jwt.verify(res.body.token, process.env.JWT_SECRET);
      expect(decoded.id).toBe(1);
      expect(decoded.role).toBe('user');
      expect(decoded.exp).toBeDefined();
    });

    it('returns 401 Unauthorized when password does not match', async () => {
      mockQuery.mockResolvedValueOnce({
        rows: [{ id: 1, email: 'user@example.com', password_hash: passwordHash, role: 'user' }],
      });

      const res = await request(app)
        .post('/login')
        .send({ email: 'user@example.com', password: 'WrongPassword' });

      expect(res.status).toBe(401);
      expect(res.body.error).toBe('Invalid credentials');
    });

    it('returns 401 Unauthorized when user is not found', async () => {
      mockQuery.mockResolvedValueOnce({ rows: [] });

      const res = await request(app)
        .post('/login')
        .send({ email: 'unknown@example.com', password: 'Password123' });

      expect(res.status).toBe(401);
      expect(res.body.error).toBe('Invalid credentials');
    });

    it('returns 400 Bad Request when request body is incomplete', async () => {
      const res = await request(app)
        .post('/login')
        .send({ email: 'user@example.com' });

      expect(res.status).toBe(400);
      expect(res.body.error).toBe('Invalid request body');
    });
  });

  describe('GET /orders (List Orders & SQL Injection Prevention)', () => {
    it('returns 401 Unauthorized when no Authorization header is provided', async () => {
      const res = await request(app).get('/orders');
      expect(res.status).toBe(401);
      expect(res.body.error).toBe('Unauthorized');
    });

    it('fetches orders using parameterized query with default status "paid"', async () => {
      const mockOrders = [
        { id: 10, customer_id: 1, status: 'paid', amount: '100.00', created_at: '2026-11-19T00:00:00Z' },
      ];
      mockQuery.mockResolvedValueOnce({ rows: mockOrders });

      const res = await request(app)
        .get('/orders')
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(200);
      expect(res.body).toEqual(mockOrders);
      expect(mockQuery).toHaveBeenCalledWith(
        'SELECT * FROM orders WHERE customer_id = $1 AND status = $2 LIMIT 20',
        [1, 'paid']
      );
    });

    it('allows valid filtered status "shipped"', async () => {
      mockQuery.mockResolvedValueOnce({ rows: [] });

      const res = await request(app)
        .get('/orders?status=shipped')
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(200);
      expect(mockQuery).toHaveBeenCalledWith(
        'SELECT * FROM orders WHERE customer_id = $1 AND status = $2 LIMIT 20',
        [1, 'shipped']
      );
    });

    it('rejects invalid status / SQL injection attempts with 400 Bad Request', async () => {
      const res = await request(app)
        .get("/orders?status=paid' OR '1'='1")
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(400);
      expect(res.body.error).toBe('Invalid status');
      expect(mockQuery).not.toHaveBeenCalled();
    });
  });

  describe('GET /orders/:id (BOLA / Ownership Verification)', () => {
    it('returns 401 Unauthorized if unauthenticated', async () => {
      const res = await request(app).get('/orders/10');
      expect(res.status).toBe(401);
    });

    it('rejects non-integer IDs with 400 Bad Request', async () => {
      const res = await request(app)
        .get('/orders/invalid-id')
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(400);
      expect(res.body.error).toBe('Invalid order ID');
    });

    it('returns order when it belongs to the authenticated customer', async () => {
      const order = { id: 10, customer_id: 1, status: 'paid', amount: '50.00', created_at: '2026-11-19T00:00:00Z' };
      mockQuery.mockResolvedValueOnce({ rows: [order] });

      const res = await request(app)
        .get('/orders/10')
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(200);
      expect(res.body).toEqual(order);
      expect(mockQuery).toHaveBeenCalledWith(
        'SELECT * FROM orders WHERE id = $1 AND customer_id = $2',
        [10, 1]
      );
    });

    it('returns 404 Not Found when order belongs to another customer (BOLA prevention)', async () => {
      // Query returns 0 rows because customer_id filter does not match
      mockQuery.mockResolvedValueOnce({ rows: [] });

      const res = await request(app)
        .get('/orders/999')
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(404);
      expect(res.body.error).toBe('Not found');
    });
  });

  describe('DELETE /admin/orders/:id (BFLA & Admin Authorization)', () => {
    it('returns 401 Unauthorized when unauthenticated', async () => {
      const res = await request(app).delete('/admin/orders/10');
      expect(res.status).toBe(401);
      expect(res.body.error).toBe('Unauthorized');
    });

    it('returns 403 Forbidden when accessed by non-admin user', async () => {
      const res = await request(app)
        .delete('/admin/orders/10')
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(403);
      expect(res.body.error).toBe('Forbidden');
      expect(mockQuery).not.toHaveBeenCalled();
    });

    it('rejects non-positive integer order IDs with 400 Bad Request', async () => {
      const res = await request(app)
        .delete('/admin/orders/abc')
        .set('Authorization', `Bearer ${adminToken}`);

      expect(res.status).toBe(400);
      expect(res.body.error).toBe('Invalid order ID');
    });

    it('allows admin to delete an existing order returning 204 No Content', async () => {
      mockQuery.mockResolvedValueOnce({ rowCount: 1 });

      const res = await request(app)
        .delete('/admin/orders/10')
        .set('Authorization', `Bearer ${adminToken}`);

      expect(res.status).toBe(204);
      expect(res.body).toEqual({});
      expect(mockQuery).toHaveBeenCalledWith('DELETE FROM orders WHERE id = $1', [10]);
    });

    it('returns 404 Not Found when admin attempts to delete a non-existent order', async () => {
      mockQuery.mockResolvedValueOnce({ rowCount: 0 });

      const res = await request(app)
        .delete('/admin/orders/9999')
        .set('Authorization', `Bearer ${adminToken}`);

      expect(res.status).toBe(404);
      expect(res.body.error).toBe('Not found');
    });
  });

  test('GET /orders rejects SQL injection in status', async () => {
    const res = await request(app)
      .get("/orders")
      .query({ status: "paid' OR '1'='1" })
      .set("Authorization", `Bearer ${userToken}`);

    expect(res.statusCode).toBe(400);
  });

  test('GET /orders accepts valid status', async () => {
    const res = await request(app)
      .get("/orders")
      .query({ status: "paid" })
      .set("Authorization", `Bearer ${userToken}`);

    expect(res.statusCode).not.toBe(400);
  });

  describe('Error Handling Middleware', () => {
    it('returns 500 with generic error and does not leak stack trace or internal error messages', async () => {
      mockQuery.mockRejectedValueOnce(new Error('Fatal Postgres connection crash'));

      const res = await request(app)
        .get('/orders')
        .set('Authorization', `Bearer ${userToken}`);

      expect(res.status).toBe(500);
      expect(res.body).toEqual({ error: 'Internal Server Error' });
      expect(res.body.stack).toBeUndefined();
    });
  });
});
