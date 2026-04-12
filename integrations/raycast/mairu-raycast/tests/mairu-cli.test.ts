import { describe, it, expect, vi, beforeEach } from "vitest";
import { runMairuCmd } from "../src/mairu-cli";
import * as child_process from "child_process";
import * as raycastApi from "@raycast/api";

// Mock child_process and util.promisify
vi.mock("child_process", () => ({
  exec: vi.fn(),
}));

// Mock util.promisify to just return our mock
vi.mock("util", () => ({
  promisify: (fn: any) => fn,
}));

// Mock @raycast/api
vi.mock("@raycast/api", () => ({
  getPreferenceValues: vi.fn(),
}));

describe("runMairuCmd", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("should construct the command with default preferences", async () => {
    vi.mocked(raycastApi.getPreferenceValues).mockReturnValue({});
    vi.mocked(child_process.exec).mockResolvedValue({ stdout: '{"success":true}', stderr: "" } as never);

    const result = await runMairuCmd("sys");
    
    expect(raycastApi.getPreferenceValues).toHaveBeenCalled();
    expect(child_process.exec).toHaveBeenCalledWith(
      expect.stringContaining('mairu sys -P "default" -o json'),
      expect.any(Object)
    );
    expect(result).toBe('{"success":true}');
  });

  it("should use preferences if provided", async () => {
    vi.mocked(raycastApi.getPreferenceValues).mockReturnValue({
      mairuCliPath: "/custom/bin/mairu",
      defaultProject: "my-project"
    });
    vi.mocked(child_process.exec).mockResolvedValue({ stdout: "[]", stderr: "" } as never);

    await runMairuCmd("memory search foo");
    
    expect(child_process.exec).toHaveBeenCalledWith(
      expect.stringContaining('/custom/bin/mairu memory search foo -P "my-project" -o json'),
      expect.any(Object)
    );
  });

  it("should override project if provided in args", async () => {
    vi.mocked(raycastApi.getPreferenceValues).mockReturnValue({
      defaultProject: "my-project"
    });
    vi.mocked(child_process.exec).mockResolvedValue({ stdout: "[]", stderr: "" } as never);

    await runMairuCmd("memory search foo", "override-project");
    
    expect(child_process.exec).toHaveBeenCalledWith(
      expect.stringContaining('mairu memory search foo -P "override-project" -o json'),
      expect.any(Object)
    );
  });

  it("should not append -o json if output format is already specified", async () => {
    vi.mocked(raycastApi.getPreferenceValues).mockReturnValue({});
    vi.mocked(child_process.exec).mockResolvedValue({ stdout: "plain text", stderr: "" } as never);

    await runMairuCmd("vibe-query 'hello' -o plain");
    
    expect(child_process.exec).toHaveBeenCalledWith(
      expect.stringContaining('mairu vibe-query \'hello\' -o plain -P "default"'),
      expect.any(Object)
    );
    expect(child_process.exec).not.toHaveBeenCalledWith(
      expect.stringContaining('-o json'),
      expect.any(Object)
    );
  });
});
