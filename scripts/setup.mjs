#!/usr/bin/env bun

import { execSync, spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

function run(cmd, opts = {}) {
  console.log(`> ${cmd}`);
  try {
    execSync(cmd, { cwd: root, stdio: "inherit", ...opts });
  } catch {
    console.error(`failed: ${cmd}`);
    process.exit(1);
  }
}

function check(name) {
  const result = spawnSync(process.platform === "win32" ? "where.exe" : "which", [name], {
    stdio: "ignore",
  });
  return result.status === 0;
}

console.log("clankerwatch setup\n");

if (!check("go")) {
  console.error("go is not installed. Install it from https://go.dev/dl/");
  process.exit(1);
}

if (!check("bun")) {
  console.error("bun is not installed. Install it from https://bun.sh/");
  process.exit(1);
}

console.log("installing dependencies...");
run("bun install");
run("bun install", { cwd: resolve(root, "web") });

console.log("\nlinking cwatch command...");
run("bun link");

console.log("\nbuilding...");
run("bun run build");

console.log(`
setup complete.

start the app:    cwatch
open the browser, create a profile, and unlock secrets.

then agents can run:
  cwatch profile list
  cwatch query <profile> --reason "..." --sql "..."
  cwatch annotate --note "..."
`);
