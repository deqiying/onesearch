// 本地验收：仅构建产物、本地 npm tarball 和 loopback fake provider；不发布、不访问真实 provider。
import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import { dirname, join, resolve } from "node:path";
import { createInterface } from "node:readline";
import { fileURLToPath } from "node:url";

const options = new Map();
for (let i = 2; i < process.argv.length; i += 2) options.set(process.argv[i], process.argv[i + 1]);
assert(options.get("--binary") && options.get("--scratch"), "usage: node scripts/test-mcp.mjs --binary <binary> --scratch <temp-dir> [--npm-cli <npm-cli.js>]");
const binary = resolve(options.get("--binary"));
const scratch = resolve(options.get("--scratch"));
mkdirSync(scratch, { recursive: true });
const work = mkdtempSync(join(scratch, "mcp-integration-"));
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const target = `${process.platform}-${process.arch}`;
assert(["win32-x64", "linux-x64", "darwin-arm64"].includes(target), `unsupported test target ${target}`);
const npmCLI = [options.get("--npm-cli"), join(dirname(process.execPath), "node_modules/npm/bin/npm-cli.js"), join(dirname(process.execPath), "../lib/node_modules/npm/bin/npm-cli.js")].find(p => p && existsSync(p));
assert(npmCLI, "npm-cli.js not found; supply --npm-cli");
const npxCLI = join(dirname(npmCLI), "npx-cli.js");
const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !/API_KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL|^ONESEARCH_/i.test(key)));
env.npm_config_cache = join(work, "npm-cache");
env.npm_config_offline = "true";
env.npm_config_update_notifier = "false";

function npm(args, cwd) {
  const result = spawnSync(process.execPath, [npmCLI, ...args], { cwd, env, encoding: "utf8", timeout: 60000 });
  assert.equal(result.status, 0, `npm ${args[0]} failed: ${result.error || result.stderr}`);
  return result.stdout;
}
const packages = join(work, "packages");
mkdirSync(packages);
cpSync(join(root, "npm/onesearch"), join(packages, "onesearch"), { recursive: true });
cpSync(join(root, "npm/packages", target), join(packages, target), { recursive: true });
mkdirSync(join(packages, target, "bin"), { recursive: true });
cpSync(binary, join(packages, target, "bin", process.platform === "win32" ? "onesearch.exe" : "onesearch"));
const tarballs = ["onesearch", target].map(name => {
  const packed = JSON.parse(npm(["pack", "--offline", "--ignore-scripts", "--json", "--pack-destination", work], join(packages, name)));
  return join(work, packed[0].filename);
});
const install = join(work, "installed");
mkdirSync(install);
writeFileSync(join(install, "package.json"), JSON.stringify({ name: "mcp-local-verification", private: true, version: "0.0.0" }));
npm(["install", "--offline", "--ignore-scripts", "--no-audit", "--no-fund", "--package-lock=false", ...tarballs], install);

const secret = "mcp-integration-fake-credential";
const calls = [];
let pending = 0;
const upstream = createServer((req, res) => {
  const url = new URL(req.url, "http://localhost");
  calls.push(url.pathname);
  assert.equal(req.headers.authorization, `Bearer ${secret}`);
  if (url.searchParams.get("query") === "wait") {
    pending++;
    res.on("close", () => pending--);
    return;
  }
  res.setHeader("Content-Type", "application/json");
  if (url.pathname === "/api/v2/libs/search") res.end(JSON.stringify({ results: [{ id: "/test/library", title: "one candidate", description: "Select explicitly" }] }));
  else if (url.pathname === "/api/v2/context") res.end(JSON.stringify({ results: [{ title: "Documentation", content: `complete body ${secret}` }] }));
  else { res.statusCode = 404; res.end("{}"); }
});
await new Promise(resolve => upstream.listen(0, "127.0.0.1", resolve));
const port = upstream.address().port;

const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(predicate, message) {
  for (let i = 0; i < 200; i++) { if (predicate()) return; await delay(25); }
  throw new Error(message);
}
const children = new Set();
function client(command, args, configDir, version) {
  const child = spawn(command, args, { cwd: install, env: { ...env, ONESEARCH_CONFIG_DIR: configDir }, stdio: ["pipe", "pipe", "pipe"] });
  children.add(child);
  const awaiting = new Map();
  let stderr = "", failed;
  child.stderr.setEncoding("utf8");
  child.stderr.on("data", text => { stderr += text; });
  const exit = new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("close", (code, signal) => { children.delete(child); resolve({ code, signal }); });
  });
  const lines = createInterface({ input: child.stdout });
  lines.on("line", line => {
    try {
      assert(Buffer.byteLength(line) < 2 * 1024 * 1024);
      assert(!line.includes(secret), "credential leaked to stdout");
      const response = JSON.parse(line);
      assert.equal(response.jsonrpc, "2.0");
      const waiter = awaiting.get(response.id);
      assert(waiter, `unexpected stdout response ${line}`);
      awaiting.delete(response.id); waiter.resolve(response);
    } catch (err) { failed = err; for (const waiter of awaiting.values()) waiter.reject(err); }
  });
  function send(id, method, params = {}) {
    if (version === "2026-07-28") params._meta = { "io.modelcontextprotocol/protocolVersion": version, "io.modelcontextprotocol/clientCapabilities": {} };
    child.stdin.write(JSON.stringify({ jsonrpc: "2.0", ...(id == null ? {} : { id }), method, params }) + "\n");
  }
  async function request(id, method, params) {
    let timer;
    try {
      return await new Promise((resolve, reject) => {
        timer = setTimeout(() => reject(new Error(`timeout ${method}; ${stderr}`)), 10000);
        awaiting.set(id, { resolve, reject }); send(id, method, params);
      });
    } finally { clearTimeout(timer); }
  }
  async function close(expectedCode = 0) {
    child.stdin.end();
    let timer;
    try {
      const result = await Promise.race([exit, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error("process did not exit after EOF")), 10000); })]);
      assert.equal(result.code, expectedCode, `exit: ${JSON.stringify(result)}; ${stderr}`);
      assert(!stderr.includes(secret));
      if (expectedCode === 0) assert.equal(stderr, "", "unexpected stderr");
      if (failed) throw failed;
    } finally { clearTimeout(timer); }
  }
  return { child, send, request, close };
}
function envelope(response, version, errorType) {
  assert(!response.error, JSON.stringify(response));
  const result = response.result;
  const text = JSON.parse(result.content[0].text);
  if (version === "2025-03-26") assert(!("structuredContent" in result));
  else assert.deepEqual(result.structuredContent, text);
  assert.equal(text.ok, !errorType);
  if (errorType) { assert.equal(text.error_type, errorType); assert.equal(result.isError, true); }
  return text;
}

try {
  const runs = [
    ...["2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"].map(version => ({ label: "binary", command: binary, args: ["mcp"], version })),
    ...["2026-07-28", "2025-11-25"].map(version => ({ label: "npx-local", command: process.execPath, args: [npxCLI, "--offline", "--no-install", "onesearch", "mcp"], version })),
  ];
  for (const [index, run] of runs.entries()) {
    const configDir = join(work, `config-${index}`);
    const c = client(run.command, run.args, configDir, run.version);
    if (run.version === "2026-07-28") assert(!(await c.request(1, "server/discover")).error);
    else {
      const response = await c.request(1, "initialize", { protocolVersion: run.version, capabilities: {}, clientInfo: { name: "independent-node-fixture", version: "1" } });
      assert.equal(response.result.protocolVersion, run.version);
      c.send(null, "notifications/initialized");
    }
    const list = await c.request("list", "tools/list");
    assert.equal(list.result.tools.length, 5);
    for (const tool of list.result.tools) assert.equal("outputSchema" in tool, run.version !== "2025-03-26");
    assert(!existsSync(configDir), "discovery created configuration");
    envelope(await c.request(3, "tools/call", { name: "onesearch_status" }), run.version);
    assert(!existsSync(configDir), "status created configuration");
    mkdirSync(configDir);
    writeFileSync(join(configDir, "config.json"), JSON.stringify({ schema_version: 1, providers: { context7: { enabled: true, base_url: `http://127.0.0.1:${port}`, api_key: secret } }, routes: {} }));
    const start = calls.length;
    const resolved = envelope(await c.request(4, "tools/call", { name: "onesearch_search", arguments: { mode: "docs", query: "q", docs_options: { library_name: "test" } } }), run.version);
    assert.equal(resolved.operation, "docs.resolve");
    assert.equal(resolved.data.needs_selection, true);
    assert.equal(calls.length, start + 1, "resolve automatically queried documentation");
    const docs = envelope(await c.request(5, "tools/call", { name: "onesearch_search", arguments: { mode: "docs", query: "q", docs_options: { library_id: "/test/library" } } }), run.version);
    assert.equal(docs.operation, "docs.query");
    assert(docs.data.content.includes("complete body"));
    assert.equal(calls.length, start + 2);
    envelope(await c.request(6, "tools/call", { name: "onesearch_search", arguments: { mode: "docs", query: "q", docs_options: { library_name: "test", library_id: "/test/library" } } }), run.version, "parameter_error");
    const cancelParams = { name: "onesearch_search", arguments: { mode: "docs", query: "wait", docs_options: { library_name: "test" } } };
    const cancelled = run.version === "2026-07-28" ? (c.send("cancel", "tools/call", cancelParams), null) : c.request("cancel", "tools/call", cancelParams);
    await until(() => pending === 1, "fake provider did not start");
    c.send(null, "notifications/cancelled", { requestId: "cancel", reason: "integration test" });
    if (cancelled) envelope(await cancelled, run.version, "cancelled");
    await until(() => pending === 0, "cancel did not close upstream request");
    assert(!(await c.request(7, "tools/list")).error, "session did not recover");
    // EOF 必须取消真实 HTTP 工作，而不只是使空闲进程退出。
    c.send("eof", "tools/call", { name: "onesearch_search", arguments: { mode: "docs", query: "wait", docs_options: { library_name: "test" } } });
    await until(() => pending === 1, "EOF fixture did not start");
    await c.close();
    await until(() => pending === 0, "EOF left an upstream request alive");
    console.log(`PASS ${run.label} ${run.version}: discovery, docs, errors, cancel, EOF`);
  }
  const malformed = spawnSync(binary, ["mcp", "--output", "secret-argument"], { env, encoding: "utf8", timeout: 10000 });
  assert.equal(malformed.status, 2); assert.equal(malformed.stdout, ""); assert(!malformed.stderr.includes("secret-argument"));
  const oversized = spawnSync(binary, ["mcp"], { env, input: "x".repeat(65537) + "\n", encoding: "utf8", timeout: 10000 });
  assert.equal(oversized.status, 5); assert.equal(oversized.stdout, "");
  console.log("PASS startup errors and 64 KiB frame limit; all packages local/offline");
} finally {
  for (const child of children) {
    child.stdin.destroy();
    if (process.platform === "win32") spawnSync("taskkill", ["/pid", String(child.pid), "/t", "/f"], { stdio: "ignore" });
    else child.kill("SIGTERM");
  }
  upstream.closeAllConnections?.();
  await new Promise(resolve => upstream.close(resolve));
}
