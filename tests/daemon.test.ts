import { afterEach, describe, expect, it, vi } from "vitest";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { CodebaseDaemon } from "../src/daemon";

type ManagerStub = {
  addContextNode: ReturnType<typeof vi.fn>;
  deleteContextNode: ReturnType<typeof vi.fn>;
};

function createManagerStub(): ManagerStub {
  return {
    addContextNode: vi.fn().mockResolvedValue(undefined),
    deleteContextNode: vi.fn().mockResolvedValue(undefined),
  };
}

function makeTempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "contextfs-daemon-test-"));
}

function source(lines: string[]): string {
  return lines.join("\n");
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("CodebaseDaemon", () => {
  it("stores rich AST context details for exported symbols", async () => {
    const tempDir = makeTempDir();
    const nestedDir = path.join(tempDir, "src", "domain");
    fs.mkdirSync(nestedDir, { recursive: true });
    const filePath = path.join(nestedDir, "feature.ts");
    const code = source([
      "export interface Profile { id: string; }",
      "export type UserId = string;",
      'export enum Mode { Read = "read", Write = "write" }',
      "export const FEATURE_FLAG = true;",
      "export function greet(name: string, loud: boolean) {",
      "  return loud ? name.toUpperCase() : name;",
      "}",
      "export class UserService {",
      "  public login(userId: UserId) {",
      "    return userId.length > 0;",
      "  }",
      "}",
    ]);
    fs.writeFileSync(filePath, code, "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);

    expect(manager.addContextNode).toHaveBeenCalledTimes(1);
    const call = manager.addContextNode.mock.calls[0];
    const [uri, name, abstractText, overviewText, content, parentUri, project, metadata, useRouter] = call;

    expect(uri).toBe("contextfs://proj/src/domain/feature.ts");
    expect(name).toBe("feature.ts");
    expect(parentUri).toBe("contextfs://proj/src/domain");
    expect(project).toBe("proj");
    expect(metadata).toEqual({ type: "file", path: filePath });
    expect(useRouter).toBe(false);

    expect(abstractText).toContain("Exports 1 classes: UserService");
    expect(abstractText).toContain("Exports 1 functions: greet");
    expect(abstractText).toContain("Exports 1 variables: FEATURE_FLAG");
    expect(abstractText).toContain("Exports 1 interfaces: Profile");
    expect(abstractText).toContain("Exports 1 enums: Mode");
    expect(abstractText).toContain("Exports 1 type aliases: UserId");

    expect(overviewText).toContain("Class UserService:");
    expect(overviewText).toContain("Methods: login");
    expect(overviewText).toContain("Function greet(name, loud)");
    expect(content).toContain("File: src/domain/feature.ts");
    expect(content).toContain("Language: ts");
    expect(content).toContain("Imports:");
    expect(content).toContain("- (none)");
    expect(content).toContain("Exports:");
    expect(content).toContain("- class UserService { methods: login }");
    expect(content).toContain("- Function greet(name, loud)");
    expect(content).toContain("- const/let/var FEATURE_FLAG");
    expect(content).toContain("- interface Profile");
    expect(content).toContain("- enum Mode");
    expect(content).toContain("- type UserId");
    expect(content).not.toContain("return userId.length > 0;");
  });

  it("falls back to default abstract and overview when no exports exist", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "helpers.ts");
    const code = source([
      "const localValue = 42;",
      "function internalOnly() {",
      "  return localValue;",
      "}",
    ]);
    fs.writeFileSync(filePath, code, "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);

    expect(manager.addContextNode).toHaveBeenCalledTimes(1);
    const [, , abstractText, overviewText, content] = manager.addContextNode.mock.calls[0];

    expect(abstractText).toBe("File helpers.ts containing source code.");
    expect(overviewText).toBe("No exported classes or functions found.");
    expect(content).toContain("File: helpers.ts");
    expect(content).toContain("Language: ts");
    expect(content).toContain("Exports:");
    expect(content).toContain("- (none)");
    expect(content).not.toContain("const localValue = 42;");
  });

  it("skips files larger than the configured size limit", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "large.ts");
    const code = source([`export const payload = "${"x".repeat(2048)}";`]);
    fs.writeFileSync(filePath, code, "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir, {
      maxFileSizeBytes: 256,
    });

    await (daemon as any).processFile(filePath);

    expect(manager.addContextNode).not.toHaveBeenCalled();
  });

  it("skips files inside ignored directories", async () => {
    const tempDir = makeTempDir();
    const ignoredFile = path.join(tempDir, "node_modules", "pkg", "index.ts");
    fs.mkdirSync(path.dirname(ignoredFile), { recursive: true });
    fs.writeFileSync(ignoredFile, source(["export const fromDependency = true;"]), "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(ignoredFile);

    expect(manager.addContextNode).not.toHaveBeenCalled();
  });

  it("removes deleted files from ts-morph project cache", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "module.ts");
    const code = source([
      "export function ping() {",
      "  return 'pong';",
      "}",
    ]);
    fs.writeFileSync(filePath, code, "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);
    expect((daemon as any).tsProject.getSourceFile(filePath)).toBeDefined();

    fs.unlinkSync(filePath);
    await (daemon as any).handleFileDelete(filePath);

    expect(manager.deleteContextNode).toHaveBeenCalledTimes(1);
    expect((daemon as any).tsProject.getSourceFile(filePath)).toBeUndefined();
  });
});
