#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");

let launcher;
try {
  launcher = require.resolve("onesearch/bin/onesearch.js");
} catch (error) {
  console.error("onesearch: missing dependency onesearch");
  console.error("Reinstall @deqiying/onesearch after publishing/installing onesearch.");
  process.exit(1);
}

const result = spawnSync(process.execPath, [launcher, ...process.argv.slice(2)], {
  stdio: "inherit",
});

if (result.error) {
  console.error("onesearch: failed to launch onesearch");
  console.error(result.error.message);
  process.exit(1);
}

if (result.signal) {
  console.error(`onesearch: terminated by signal ${result.signal}`);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
