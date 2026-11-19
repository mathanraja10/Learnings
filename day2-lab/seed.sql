CREATE TABLE orders (
  id SERIAL PRIMARY KEY,
  customer_id INT NOT NULL,
  status TEXT NOT NULL,
  amount NUMERIC(10,2) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

INSERT INTO orders (customer_id, status, amount, created_at)
SELECT
  (random() * 50000)::int,
  (ARRAY['pending','paid','shipped','cancelled'])[1 + (random() * 3)::int],
  (random() * 5000)::numeric(10,2),
  now() - random() * interval '365 days'
FROM generate_series(1, 1000000);

ANALYZE orders;