const http = require("http");
const fs = require("fs");
const path = require("path");

const PORT = process.env.PORT || 8000;
const INDEX_FILE = path.join(__dirname, "public", "index.html");

const operations = {
  "/add": { name: "addition", fn: (a, b) => a + b },
  "/subtract": { name: "subtraction", fn: (a, b) => a - b },
  "/multiply": { name: "multiplication", fn: (a, b) => a * b },
  "/divide": { name: "division", fn: (a, b) => a / b }
};

function sendJson(res, statusCode, body) {
  res.statusCode = statusCode;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(body));
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);

  // UI
  if (url.pathname === "/" || url.pathname === "/index.html") {
    return fs.readFile(INDEX_FILE, (err, html) => {
      if (err) {
        return sendJson(res, 500, { error: "UI file not found" });
      }
      res.setHeader("Content-Type", "text/html; charset=utf-8");
      res.end(html);
    });
  }

  // API info
  if (url.pathname === "/api") {
    return sendJson(res, 200, {
      message: "Node Calculator API",
      endpoints: Object.keys(operations),
      example: "/add?a=10&b=20"
    });
  }

  // Calculator operations
  const op = operations[url.pathname];
  if (op) {
    const rawA = url.searchParams.get("a");
    const rawB = url.searchParams.get("b");
    const a = Number(rawA);
    const b = Number(rawB);

    if (rawA === null || rawB === null || rawA.trim() === "" || rawB.trim() === "" ||
        Number.isNaN(a) || Number.isNaN(b)) {
      return sendJson(res, 400, { error: "Both 'a' and 'b' must be valid numbers" });
    }

    if (url.pathname === "/divide" && b === 0) {
      return sendJson(res, 400, { error: "Cannot divide by zero" });
    }

    return sendJson(res, 200, {
      operation: op.name,
      a,
      b,
      result: op.fn(a, b)
    });
  }

  sendJson(res, 404, { error: "Route not found" });
});

server.listen(PORT, () => {
  console.log(`Calculator running on http://localhost:${PORT}`);
});
