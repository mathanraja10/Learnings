const { parentPort } = require("node:worker_threads");

const end = Date.now() + 2000;

while (Date.now() < end) {}

parentPort.postMessage("report done");