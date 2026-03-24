import { describe, expect, it, vi } from "vitest";
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
  return fs.mkdtempSync(path.join(os.tmpdir(), "contextfs-parallel-test-"));
}

describe("CodebaseDaemon parallel processing", () => {
  it("processes multiple files concurrently during initial scan", async () => {
    const tempDir = makeTempDir();
    const fileCount = 20;
    for (let i = 0; i < fileCount; i++) {
      fs.writeFileSync(
        path.join(tempDir, `module${i}.ts`),
        `export function fn${i}() { return ${i}; }`,
        "utf8"
      );
    }

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir, {
      concurrency: 4,
    });

    await (daemon as any).processAllFiles();

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(fileCount);
  });

  it("processes pending file batch concurrently", async () => {
    const tempDir = makeTempDir();
    const files: string[] = [];
    for (let i = 0; i < 10; i++) {
      const p = path.join(tempDir, `change${i}.ts`);
      fs.writeFileSync(p, `export const v${i} = ${i};`, "utf8");
      files.push(p);
    }

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir, {
      concurrency: 4,
    });

    for (const f of files) {
      (daemon as any).pendingFiles.add(path.resolve(f));
    }
    await (daemon as any).processPendingFiles();

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(10);
  });
});
