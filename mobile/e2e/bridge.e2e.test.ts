/**
 * E2E: drives the real `mairu acp-bridge` binary from the mobile transport
 * + client. Skipped automatically when the bridge package isn't present in
 * this checkout (e.g. the feat/mairu-mobile branch on its own); will start
 * running once feat/acp-bridge merges into master.
 */
import { spawn, ChildProcessWithoutNullStreams, execSync } from "child_process";
import * as fs from "fs";
import * as path from "path";
import * as net from "net";
import WS from "ws";

import { WSTransport } from "../src/acp/transport";
import { ACPClient } from "../src/acp/client";
import { createSession } from "../src/api/sessions";

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const BRIDGE_PKG = path.join(REPO_ROOT, "mairu", "internal", "acpbridge");
const HAS_BRIDGE = fs.existsSync(BRIDGE_PKG);

const describeIfBridge = HAS_BRIDGE ? describe : describe.skip;

async function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.unref();
    srv.on("error", reject);
    srv.listen(0, () => {
      const addr = srv.address();
      if (typeof addr === "object" && addr) {
        const p = addr.port;
        srv.close(() => resolve(p));
      } else {
        srv.close();
        reject(new Error("could not allocate port"));
      }
    });
  });
}

async function waitForHttp(host: string, timeoutMs: number) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const r = await fetch(`${host}/sessions`);
      if (r.ok) return;
    } catch {
      /* not ready */
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`bridge did not become ready at ${host}`);
}

describeIfBridge("mairu-mobile vs real acp-bridge", () => {
  let bridge: ChildProcessWithoutNullStreams;
  let host: string;

  beforeAll(async () => {
    execSync("make build", { cwd: path.join(REPO_ROOT, "mairu"), stdio: "inherit" });
    const port = await freePort();
    const bin = path.join(REPO_ROOT, "mairu", "bin", "mairu");
    bridge = spawn(bin, ["acp-bridge", "--addr", `127.0.0.1:${port}`, "--no-tailscale"]);
    bridge.stdout.on("data", (b) => process.stdout.write(`[bridge] ${b}`));
    bridge.stderr.on("data", (b) => process.stderr.write(`[bridge] ${b}`));
    host = `http://127.0.0.1:${port}`;
    await waitForHttp(host, 5000);
    (globalThis as any).WebSocket = WS;
  }, 60_000);

  afterAll(() => {
    bridge?.kill();
  });

  test("create session, attach, prompt, receive a frame", async () => {
    const id = await createSession(host, "mairu");
    expect(id).toBeTruthy();

    const wsBase = host.replace(/^http/, "ws") + "/acp";
    const t = new WSTransport({ baseUrl: wsBase, sessionId: id });
    const c = new ACPClient(t);

    let gotAny = false;
    c.onNotification(() => {
      gotAny = true;
    });

    t.connect();
    await new Promise((r) => setTimeout(r, 500));

    c.notify("session/prompt", { text: "say hi" });

    for (let i = 0; i < 50 && !gotAny; i++) {
      await new Promise((r) => setTimeout(r, 100));
    }
    t.disconnect();
    expect(gotAny).toBe(true);
  }, 30_000);
});

if (!HAS_BRIDGE) {
  // Keep jest happy when the suite is fully skipped: a top-level test that's
  // skipped just emits a friendly note.
  test.skip("acpbridge not in this checkout — skipping e2e", () => {});
}
