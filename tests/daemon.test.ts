import { afterEach, describe, expect, it, vi } from "vitest";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { CodebaseDaemon } from "../src/daemon";

type ManagerStub = {
  upsertFileContextNode: ReturnType<typeof vi.fn>;
  deleteContextNode: ReturnType<typeof vi.fn>;
};

function createManagerStub(): ManagerStub {
  return {
    upsertFileContextNode: vi.fn().mockResolvedValue(undefined),
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
  it("stores NL content with abstract summary, compact overview, and NL body", async () => {
    const tempDir = makeTempDir();
    const nestedDir = path.join(tempDir, "src", "domain");
    fs.mkdirSync(nestedDir, { recursive: true });
    const filePath = path.join(nestedDir, "feature.ts");
    const code = source([
      "import { slugify } from './slug';",
      "const INTERNAL_SEED = 42;",
      "",
      "function normalize(name: string) {",
      "  return slugify(`${name}-${INTERNAL_SEED}`);",
      "}",
      "",
      "export function greet(name: string) {",
      "  return normalize(name);",
      "}",
      "",
      "export class UserService {",
      "  public run(input: string) {",
      "    this.bump();",
      "    return greet(input);",
      "  }",
      "  private bump() {",
      "    return normalize('x');",
      "  }",
      "}",
    ]);
    fs.writeFileSync(filePath, code, "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(1);
    const call = manager.upsertFileContextNode.mock.calls[0];
    const [uri, name, abstractText, overviewText, content, parentUri, project, metadata] = call;

    expect(uri).toBe("contextfs://proj/src/domain/feature.ts");
    expect(name).toBe("feature.ts");
    expect(parentUri).toBe("contextfs://proj/src/domain");
    expect(project).toBe("proj");

    // abstract = NL file summary (no longer empty)
    expect(abstractText).toBeTruthy();
    expect(abstractText).toMatch(/greet|UserService|normalize/i);

    // overview = compact graph notation (was previously in content)
    expect(overviewText).toContain("File: src/domain/feature.ts");
    expect(overviewText).toContain("Language: ts");
    expect(overviewText).toContain("LogicGraph: v1");
    expect(overviewText).toContain("Symbols:");
    expect(overviewText).toContain("- fn fn:greet");
    expect(overviewText).toContain("- fn fn:normalize");
    expect(overviewText).toContain("- mtd mtd:UserService.run");
    expect(overviewText).toContain("Edges:");
    expect(overviewText).toContain("- call fn:greet -> fn:normalize");
    expect(overviewText).toContain("- call mtd:UserService.run -> mtd:UserService.bump");
    expect(overviewText).toContain("- import file -> module:./slug");

    // content = NL descriptions (no longer compact notation)
    expect(content).toMatch(/greet/i);
    expect(content).toMatch(/normalize/i);
    expect(content).toMatch(/UserService/i);
    expect(content).not.toContain("- fn fn:greet"); // compact notation is in overview now

    // metadata now always includes logic_graph
    expect(metadata.type).toBe("file");
    expect(metadata.path).toBe(filePath);
    expect(metadata.logic_graph).toBeDefined();
  });

  it("falls back to default abstract and overview when no declarations exist", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "helpers.ts");
    const code = source([
      "/* empty module */",
    ]);
    fs.writeFileSync(filePath, code, "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(1);
    const [, , abstractText, overviewText, content, , , metadata] = manager.upsertFileContextNode.mock.calls[0];

    // abstract = NL file summary (fallback for empty file)
    expect(abstractText).toBeTruthy();
    expect(abstractText).toMatch(/empty|declaration/i);

    // overview = compact graph with (none)
    expect(overviewText).toContain("File: helpers.ts");
    expect(overviewText).toContain("Language: ts");
    expect(overviewText).toContain("Symbols:");
    expect(overviewText).toContain("- (none)");
    expect(overviewText).toContain("Edges:");

    // content = minimal NL (empty or near-empty since no symbols)
    expect(content).toBeDefined();

    // metadata includes logic_graph
    expect(metadata.type).toBe("file");
    expect(metadata.logic_graph).toBeDefined();
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

    expect(manager.upsertFileContextNode).not.toHaveBeenCalled();
  });

  it("skips files inside ignored directories", async () => {
    const tempDir = makeTempDir();
    const ignoredFile = path.join(tempDir, "node_modules", "pkg", "index.ts");
    fs.mkdirSync(path.dirname(ignoredFile), { recursive: true });
    fs.writeFileSync(ignoredFile, source(["export const fromDependency = true;"]), "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(ignoredFile);

    expect(manager.upsertFileContextNode).not.toHaveBeenCalled();
  });

  it("removes deleted files and clears internal caches", async () => {
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
    expect((daemon as any).fileContentHashes.has(path.resolve(filePath))).toBe(true);

    fs.unlinkSync(filePath);
    await (daemon as any).handleFileDelete(filePath);

    expect(manager.deleteContextNode).toHaveBeenCalledTimes(1);
    expect((daemon as any).fileContentHashes.has(path.resolve(filePath))).toBe(false);
    expect((daemon as any).fileFingerprints.has(path.resolve(filePath))).toBe(false);
    expect((daemon as any).nodePayloadHashes.has(path.resolve(filePath))).toBe(false);
  });

  it("skips mtime-only changes when file content is unchanged", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "module.ts");
    fs.writeFileSync(
      filePath,
      source([
        "export function ping() {",
        "  return 'pong';",
        "}",
      ]),
      "utf8"
    );

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);
    fs.utimesSync(filePath, new Date(Date.now() + 1000), new Date(Date.now() + 1000));
    await (daemon as any).processFile(filePath);

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(1);
  });

  it("re-upserts when NL content changes even if compact graph is identical", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "math.ts");
    fs.writeFileSync(
      filePath,
      source([
        "export function add(a: number, b: number) {",
        "  return a + b;",
        "}",
      ]),
      "utf8"
    );

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);
    fs.writeFileSync(
      filePath,
      source([
        "export function add(a: number, b: number) {",
        "  const sum = a + b;",
        "  return sum;",
        "}",
      ]),
      "utf8"
    );
    await (daemon as any).processFile(filePath);

    // With NL content, the body descriptions differ, so both upserts happen
    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(2);
  });

  it("enforces compact payload bounds with truncation markers", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "huge.ts");
    const lines: string[] = [];
    for (let i = 0; i < 180; i += 1) {
      lines.push(`export function f${i}(value: number) { return value + ${i}; }`);
    }
    fs.writeFileSync(filePath, source(lines), "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);

    await (daemon as any).processFile(filePath);

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(1);
    const [, , abstractText, overviewText, content, , , metadata] = manager.upsertFileContextNode.mock.calls[0];

    // metadata includes logic_graph
    expect(metadata.type).toBe("file");
    expect(metadata.path).toBe(filePath);
    expect(metadata.logic_graph).toBeDefined();

    // overview = compact graph with truncation info
    expect(overviewText).toContain("GraphStats:");
    expect(overviewText).toContain("Truncated:");

    // overview should be bounded
    expect(overviewText.length).toBeLessThanOrEqual(16_100);

    // abstract should be a NL summary
    expect(abstractText).toBeTruthy();

    // content = NL descriptions, may have truncation marker
    expect(content).toBeDefined();
  });

  it("produces NL content with abstract summary, compact overview, and NL body", async () => {
    const tempDir = makeTempDir();
    const filePath = path.join(tempDir, "service.ts");
    const code = source([
      "export function validate(input: string) {",
      "  if (!input) {",
      "    throw new Error('Required');",
      "  }",
      "  return input.trim();",
      "}",
      "",
      "export function process(data: string) {",
      "  const clean = validate(data);",
      "  return clean.toUpperCase();",
      "}",
    ]);
    fs.writeFileSync(filePath, code, "utf8");

    const manager = createManagerStub();
    const daemon = new CodebaseDaemon(manager as any, "proj", tempDir);
    await (daemon as any).processFile(filePath);

    expect(manager.upsertFileContextNode).toHaveBeenCalledTimes(1);
    const [, , abstractText, overviewText, content] = manager.upsertFileContextNode.mock.calls[0];

    // abstract = NL file summary
    expect(abstractText).toBeTruthy();
    expect(abstractText).toMatch(/validate|process/i);

    // overview = compact graph notation
    expect(overviewText).toContain("Symbols:");
    expect(overviewText).toContain("Edges:");
    expect(overviewText).toContain("fn fn:validate");

    // content = NL descriptions
    expect(content).toMatch(/validate/i);
    expect(content).toMatch(/[Tt]hrows|[Rr]eturns/);
    expect(content).toMatch(/process/i);
    expect(content).toMatch(/[Cc]all.*validate/);
  });
});
