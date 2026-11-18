const express = require("express");
const { monitorEventLoopDelay } = require("node:perf_hooks");
const { Worker } = require("node:worker_threads");

const eventLoopMonitor = monitorEventLoopDelay({
  resolution: 20,
});

eventLoopMonitor.enable();

function asyncHandler(fn) {
  return (req, res, next) => {
    Promise.resolve(fn(req, res, next)).catch(next);
  };
}

const app = express();

// Fake DB
const db = {
  getUser: (id) =>
    new Promise((resolve, reject) =>
      setTimeout(() => {
        if (id === "0") {
          reject(new Error("User not found"));
        } else {
          resolve({
            id,
            name: "Madhan",
          });
        }
      }, 100)
    ),
};

// BUG 1
app.get("/users/:id", async (req, res) => {
  const user = await db.getUser(req.params.id);

  res.json({ name: user.name });
});

// BUG 2
app.get(
  "/crash/:id",
  asyncHandler(async (req, res) => {
    const user = await db.getUser(req.params.id);
    res.json(user);
  })
);
// BUG 3
// app.get("/report", (req, res) => {
//   const end = Date.now() + 2000;

//   while (Date.now() < end) {}

//   res.send("report done");
// });

app.get("/report", (req, res) => {
  const worker = new Worker("./report-worker.js");

  worker.on("message", (message) => {
    res.send(message);
  });

  worker.on("error", (error) => {
    res.status(500).send(error.message);
  });
});

app.get("/ping", (req, res) => {
  res.send("pong");
});

app.get("/event-loop", (req, res) => {
  res.json({
    maxDelayMs: Number(eventLoopMonitor.max) / 1e6,
    meanDelayMs: Number(eventLoopMonitor.mean) / 1e6,
  });
});

app.use((err, req, res, next) => {
  console.error("GLOBAL ERROR:", err.message);

  res.status(500).json({
    error: err.message,
  });
});

app.use((err, req, res, next) => {
  console.error("GLOBAL ERROR:", err.message);

  res.status(500).json({
    error: err.message,
  });
});

app.listen(3000, () => {
  console.log("API on :3000");
});