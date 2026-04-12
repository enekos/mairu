import { exec } from "child_process";
import { promisify } from "util";
import { getPreferenceValues } from "@raycast/api";

const execAsync = promisify(exec);

interface Preferences {
  mairuCliPath?: string;
  defaultProject?: string;
}

export async function runMairuCmd(
  args: string,
  project?: string,
  cwd?: string,
): Promise<string> {
  const prefs = getPreferenceValues<Preferences>();
  const cliPath = prefs.mairuCliPath || "mairu";
  const proj = project || prefs.defaultProject || "default";

  let fullCmd = `${cliPath} ${args} -P "${proj}"`;
  if (!args.includes("-o ")) {
    fullCmd += ` -o json`;
  }

  try {
    const defaultPaths = [
      "/usr/local/bin",
      "/opt/homebrew/bin",
      `${process.env.HOME}/go/bin`,
      `${process.env.HOME}/.local/bin`,
    ].join(":");

    const { stdout, stderr } = await execAsync(fullCmd, {
      env: {
        ...process.env,
        PATH: `${process.env.PATH || ""}:${defaultPaths}`,
      },
      cwd: cwd || process.cwd(),
    });
    if (stderr && !stdout) {
      // Mairu returned stderr without stdout; error will be thrown below if needed
    }
    return stdout;
  } catch (error: Error | unknown) {
    const err = error as { message?: string; code?: number | string; stdout?: string };
    // Error running mairu; details included in thrown error

    if (err.code === 127 || (err.message && err.message.includes("command not found"))) {
      throw new Error(
        `Mairu CLI not found. Please set the absolute path to the 'mairu' executable in the Raycast extension preferences (e.g., /usr/local/bin/mairu or /Users/username/go/bin/mairu).`,
      );
    }

    if (err.stdout) {
      return err.stdout as string; // Mairu might return JSON even on failure
    }
    throw error;
  }
}
