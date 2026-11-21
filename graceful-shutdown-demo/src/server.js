import express from "express";

const app = express();

const PORT = 5000;

let isShuttingDown = false;
console.log("Hello 1");

// Normal API
app.get("/", (req, res) => {
    res.json({
        message: "Hello from Node Docker container",
    });
});

// Health check
app.get("/health", (req, res) => {
    if (isShuttingDown) {
        return res.status(503).json({
            status: "shutting_down",
        });
    }

    res.status(200).json({
        status: "ok",
    });
});

// Long-running request
app.get("/slow", async (req, res) => {
    console.log("Slow request started");

    await new Promise((resolve) => {
        setTimeout(resolve, 15000);
    });

    console.log("Slow request finished");

    res.json({
        message: "Slow request completed",
    });
});

const server = app.listen(PORT, () => {
    console.log(`Server running on port ${PORT}`);
});

/*
 * Graceful shutdown
 */
function shutdown(signal) {
    console.log(`\n${signal} received`);

    isShuttingDown = true;

    console.log("Stopping new requests...");

    server.close(async () => {
        console.log("HTTP server closed");

        // Simulate cleanup work
        await new Promise((resolve) => setTimeout(resolve, 1000));

        console.log("Cleanup completed");
        console.log("Exiting process...");

        process.exit(0);
    });
}

process.on("SIGTERM", () => {
    shutdown("SIGTERM");
});

process.on("SIGINT", () => {
    shutdown("SIGINT");
});