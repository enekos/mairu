import path from "path";

export function resolveDefaultProject(workspacePath: string | undefined): string {
  if (!workspacePath) return "default";
  const base = path.basename(workspacePath.trim());
  return base || "default";
}
