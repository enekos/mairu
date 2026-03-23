import { spawn } from "child_process";

export interface RunCliOptions {
  cliPath: string;
  args: string[];
  timeoutMs: number;
  stdinText?: string;
}

export interface RunCliResult {
  stdout: string;
  stderr: string;
  exitCode: number;
  commandString: string;
}

export class CliRunError extends Error {
  public readonly commandString: string;
  public readonly stderr: string;
  public readonly exitCode: number;

  constructor(message: string, commandString: string, stderr: string, exitCode: number) {
    super(message);
    this.name = "CliRunError";
    this.commandString = commandString;
    this.stderr = stderr;
    this.exitCode = exitCode;
  }
}

export function runContextCli(options: RunCliOptions): Promise<RunCliResult> {
  const commandString = [options.cliPath, ...options.args].join(" ");

  return new Promise((resolve, reject) => {
    const child = spawn(options.cliPath, options.args, {
      stdio: "pipe",
      shell: false,
    });

    let stdout = "";
    let stderr = "";
    let timedOut = false;

    const timer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGTERM");
    }, options.timeoutMs);

    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString();
    });

    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });

    child.on("error", (err) => {
      clearTimeout(timer);
      reject(
        new CliRunError(
          `Failed to start context-cli: ${err.message}`,
          commandString,
          stderr.trim(),
          -1
        )
      );
    });

    child.on("close", (code) => {
      clearTimeout(timer);

      if (timedOut) {
        reject(
          new CliRunError(
            `context-cli timed out after ${options.timeoutMs}ms`,
            commandString,
            stderr.trim(),
            code ?? -1
          )
        );
        return;
      }

      const exitCode = code ?? -1;
      if (exitCode !== 0) {
        reject(
          new CliRunError(
            `context-cli exited with code ${exitCode}`,
            commandString,
            stderr.trim(),
            exitCode
          )
        );
        return;
      }

      resolve({
        stdout,
        stderr,
        exitCode,
        commandString,
      });
    });

    if (options.stdinText !== undefined) {
      child.stdin.write(options.stdinText);
    }
    child.stdin.end();
  });
}
