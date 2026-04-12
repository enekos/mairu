import { Detail, ActionPanel, Action } from "@raycast/api";
import { useState, useEffect } from "react";
import { runMairuCmd } from "./mairu-cli";

export default function Command() {
  const [isLoading, setIsLoading] = useState(true);
  const [status, setStatus] = useState<string>("Loading system status...");

  useEffect(() => {
    async function fetchStatus() {
      try {
        const stdout = await runMairuCmd(`sys -o json`);
        setStatus(stdout);
      } catch (error: Error | unknown) {
        setStatus(`Error fetching status:\n${(error as Error).message}`);
      } finally {
        setIsLoading(false);
      }
    }

    fetchStatus();
  }, []);

  return (
    <Detail
      isLoading={isLoading}
      markdown={`\`\`\`json\n${status}\n\`\`\``}
      actions={
        <ActionPanel>
          <Action.CopyToClipboard title="Copy Status" content={status} />
        </ActionPanel>
      }
    />
  );
}
