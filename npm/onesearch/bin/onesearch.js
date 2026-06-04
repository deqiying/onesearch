#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");

const targets = {
  "darwin-arm64": {
    packageName: "@deqiying/onesearch-darwin-arm64",
    binaryPath: "bin/onesearch",
  },
  "linux-x64": {
    packageName: "@deqiying/onesearch-linux-x64",
    binaryPath: "bin/onesearch",
  },
  "win32-x64": {
    packageName: "@deqiying/onesearch-win32-x64",
    binaryPath: "bin/onesearch.exe",
  },
};

const targetKey = `${process.platform}-${process.arch}`;
const target = targets[targetKey];

if (!target) {
  console.error(`onesearch: unsupported platform ${targetKey}`);
  console.error(`Supported platforms: ${Object.keys(targets).join(", ")}`);
  process.exit(1);
}

let binary;
try {
  binary = require.resolve(`${target.packageName}/${target.binaryPath}`);
} catch (error) {
  console.error(`onesearch: missing optional package ${target.packageName}`);
  console.error("Reinstall onesearch with optional dependencies enabled.");
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(`onesearch: failed to launch ${binary}`);
  console.error(result.error.message);
  process.exit(1);
}

if (result.signal) {
  console.error(`onesearch: terminated by signal ${result.signal}`);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
