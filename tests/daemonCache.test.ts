import { afterEach, describe, expect, it, vi } from "vitest";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { CodebaseDaemon } from "../src/daemon";

function createManagerStub() {
  return {
    upsertFileContextNode: vi.fn().mockResolvedValue(undefined),
    deleteContextNode: vi.fn().mockResolvedValue(undefined),
  };
}

function makeTempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "contextfs-cache-test-"));
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Persistent hash cache", () => {
  it("saves cache file after processing", async () => {
    const tempDir = makeTempDir();
    fs.writeFileSync(
      path.join(tempDir, "mod.ts"),
      "export function hello() { return 'hi'; }",
      "utf8"
    );

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processAllFiles();
    (daemon as any).saveCache();

    const cachePath = path.join(tempDir, ".contextfs-cache.json");
    expect(fs.existsSync(cachePath)).toBe(true);

    const cache = JSON.parse(fs.readFileSync(cachePath, "utf8"));
    expect(cache.version).toBe(1);
    expect(Object.keys(cache.files).length).toBe(1);
  });

  it("skips unchanged files on restart using loaded cache", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "mod.ts");
    fs.writeFileSync(filePath, "export function hello() { return 'hi'; }", "utf8");

    const manager1 = createManagerStub();
    const daemon1 = new CodebaseDaemon(manager1 as any, "proj", tempDir);
    await (daemon1 as any).processAllFiles();
    (daemon1 as any).saveCache();
    expect(manager1.upsertFileContextNode).toHaveBeenCalledTimes(1);

    const manager2 = createManagerStub();
    const daemon2 = new CodebaseDaemon(manager2 as any, "proj", tempDir);
    (daemon2 as any).loadCache();
    await (daemon2 as any).processAllFiles();

    expect(manager2.upsertFileContextNode).not.toHaveBeenCalled();
  });

  it("reprocesses files when content changes between runs", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "mod.ts");
    fs.writeFileSync(filePath, "export function hello() { return 'hi'; }", "utf8");

    const manager1 = createManagerStub();
    const daemon1 = new CodebaseDaemon(manager1 as any, "proj", tempDir);
    await (daemon1 as any).processAllFiles();
    (daemon1 as any).saveCache();

    fs.writeFileSync(filePath, "export function hello() { return 'changed'; }", "utf8");

    const manager2 = createManagerStub();
    const daemon2 = new CodebaseDaemon(manager2 as any, "proj", tempDir);
    (daemon2 as any).loadCache();
    await (daemon2 as any).processAllFiles();

    expect(manager2.upsertFileContextNode).toHaveBeenCalledTimes(1);
  });

  it("ignores cache with mismatched version", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "mod.ts");
    fs.writeFileSync(filePath, "export function hello() { return 'hi'; }", "utf8");

    const cachePath = path.join(tempDir, ".contextfs-cache.json");
    fs.writeFileSync(cachePath, JSON.stringify({ version: 999, files: {} }), "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);
    (daemon as any).loadCache();
    await (daemon as any).processAllFiles();

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(1);
  });
});
