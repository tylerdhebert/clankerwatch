#!/usr/bin/env bun

import { spawn } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const rawMode = args.includes("--raw");
const appArgs = args.filter((arg) => arg !== "--raw");

const SHUTDOWN_GRACE_MS = 8_000;
const SHUTDOWN_ESCALATE_MS = 3_000;
const SHUTDOWN_FORCE_WAIT_MS = 2_000;

function childExited(child) {
  return child.exitCode !== null || child.signalCode !== null;
}

function waitForChildExit(child) {
  if (childExited(child)) return Promise.resolve();
  return new Promise((resolve) => {
    child.once("exit", resolve);
    child.once("error", resolve);
  });
}

function signalChild(child, signal) {
  if (childExited(child)) return;
  try {
    child.kill(signal);
  } catch {
    // Process already gone.
  }
}

function forceKillChild(child) {
  if (childExited(child)) return;
  signalChild(child, "SIGKILL");
  if (process.platform === "win32" && child.pid) {
    spawn("taskkill", ["/PID", String(child.pid), "/T", "/F"], {
      stdio: "ignore",
      shell: false,
    }).unref();
  }
}

async function shutdownChildren(children, { force = false } = {}) {
  const pending = [...children].filter((child) => !childExited(child));
  if (pending.length === 0) return;

  const signal = force ? "SIGKILL" : "SIGINT";
  for (const child of pending) {
    signalChild(child, signal);
  }

  const graceMs = force ? SHUTDOWN_FORCE_WAIT_MS : SHUTDOWN_GRACE_MS;
  await Promise.race([
    Promise.all(pending.map(waitForChildExit)),
    new Promise((resolve) => setTimeout(resolve, graceMs)),
  ]);

  let survivors = pending.filter((child) => !childExited(child));
  if (survivors.length === 0 || force) return;

  for (const child of survivors) {
    signalChild(child, "SIGTERM");
  }
  await Promise.race([
    Promise.all(survivors.map(waitForChildExit)),
    new Promise((resolve) => setTimeout(resolve, SHUTDOWN_ESCALATE_MS)),
  ]);

  survivors = survivors.filter((child) => !childExited(child));
  for (const child of survivors) {
    forceKillChild(child);
  }
  await Promise.race([
    Promise.all(survivors.map(waitForChildExit)),
    new Promise((resolve) => setTimeout(resolve, SHUTDOWN_FORCE_WAIT_MS)),
  ]);
}

if (appArgs.length === 0 || appArgs[0] === "app" || appArgs[0] === "dev") {
  await startApp();
} else {
  await runGo(appArgs);
}

async function runGo(goArgs) {
  const child = spawn("go", ["run", "./cmd/cwatch", ...goArgs], {
    cwd: root,
    stdio: "inherit",
    shell: false,
  });
  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 1);
  });
}

async function startApp() {
  const children = new Set();
  let viteStarted = false;
  let opened = false;
  let shuttingDown = false;
  const noOpen = process.env.CWATCH_NO_OPEN === "1";
  const appUrl = rawMode ? "http://127.0.0.1:5173/" : "https://cwatch.localhost/";

  if (!rawMode) {
    await trustPortless();
  }

  const apiCommand = rawMode
    ? ["go", "run", "./cmd/cwatch", "serve"]
    : ["bun", "x", "portless", "cwatchapi", "go", "run", "./cmd/cwatch", "serve"];
  const api = spawn(apiCommand[0], apiCommand.slice(1), {
    cwd: root,
    stdio: ["ignore", "pipe", "pipe"],
    shell: false,
  });
  children.add(api);

  api.stderr.on("data", (chunk) => process.stderr.write(chunk));
  api.stdout.setEncoding("utf8");
  api.stdout.on("data", (chunk) => {
    for (const line of chunk.split(/\r?\n/)) {
      if (!line) continue;
      if (line.startsWith("CWATCH_SERVER ")) {
        startVite();
      } else {
        process.stdout.write(`${line}\n`);
      }
    }
  });

  api.on("exit", (code) => {
    children.delete(api);
    if (shuttingDown) return;
    if (!viteStarted) {
      process.exit(code ?? 1);
      return;
    }
    void shutdownChildren(children).then(() => process.exit(code ?? 1));
  });

  function startVite() {
    if (viteStarted) return;
    viteStarted = true;
    const webCommand = rawMode
      ? ["bun", "run", "--cwd", "web", "dev", "--host", "127.0.0.1", "--port", "5173"]
      : ["bun", "x", "portless", "cwatch", "bun", "run", "--cwd", "web", "dev", "--host", "127.0.0.1"];
    const web = spawn(webCommand[0], webCommand.slice(1), {
      cwd: root,
      stdio: ["ignore", "pipe", "pipe"],
      shell: false,
    });
    children.add(web);

    const maybeOpen = () => {
      if (opened) return;
      opened = true;
      if (noOpen) {
        process.stdout.write(`clankerwatch app ready at ${appUrl}\n`);
        return;
      }
      setTimeout(() => openBrowser(appUrl), 700);
    };

    web.stdout.on("data", (chunk) => {
      const text = chunk.toString();
      process.stdout.write(text);
      if (text.includes("Local:") || text.includes("ready in")) {
        maybeOpen();
      }
    });
    web.stderr.on("data", (chunk) => process.stderr.write(chunk));
    web.on("exit", (code) => {
      children.delete(web);
      if (shuttingDown) return;
      void shutdownChildren(children).then(() => process.exit(code ?? 0));
    });
    setTimeout(maybeOpen, 1400);
  }

  const shutdown = async (force = false) => {
    if (shuttingDown && !force) return;
    shuttingDown = true;
    await shutdownChildren(children, { force });
    process.exit(0);
  };

  const onStop = () => {
    void shutdown(shuttingDown);
  };
  process.on("SIGINT", onStop);
  process.on("SIGTERM", onStop);
}

async function trustPortless() {
  await new Promise((resolve) => {
    const child = spawn("bun", ["x", "portless", "trust"], {
      cwd: root,
      stdio: "inherit",
      shell: false,
    });
    child.on("exit", () => resolve());
    child.on("error", (error) => {
      process.stderr.write(`portless trust preflight failed: ${error.message}\n`);
      resolve();
    });
  });
}

function openBrowser(url) {
  const platform = process.platform;
  if (platform === "win32") {
    spawn("cmd", ["/c", "start", "", url], { detached: true, stdio: "ignore" }).unref();
    return;
  }
  if (platform === "darwin") {
    spawn("open", [url], { detached: true, stdio: "ignore" }).unref();
    return;
  }
  spawn("xdg-open", [url], { detached: true, stdio: "ignore" }).unref();
}
