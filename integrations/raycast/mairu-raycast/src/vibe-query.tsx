import {
  Form,
  ActionPanel,
  Action,
  showToast,
  Toast,
  Detail,
} from "@raycast/api";
import { useState } from "react";
import { runMairuCmd } from "./mairu-cli";

export default function Command() {
  const [isLoading, setIsLoading] = useState(false);
  const [result, setResult] = useState<string | null>(null);

  async function handleSubmit(values: { query: string }) {
    setIsLoading(true);
    try {
      const cmd = `vibe-query "${values.query.replace(/"/g, '\\"')}" -o plain`;
      const stdout = await runMairuCmd(cmd);
      setResult(stdout);
    } catch (error: Error | unknown) {
      showToast({
        style: Toast.Style.Failure,
        title: "Query failed",
        message: (error as Error).message,
      });
      setResult(`Error: ${(error as Error).message}`);
    } finally {
      setIsLoading(false);
    }
  }

  if (result) {
    return (
      <Detail
        markdown={result}
        actions={
          <ActionPanel>
            <Action title="New Query" onAction={() => setResult(null)} />
            <Action.CopyToClipboard title="Copy Result" content={result} />
          </ActionPanel>
        }
      />
    );
  }

  return (
    <Form
      isLoading={isLoading}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Execute Query" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextArea
        id="query"
        title="Query"
        placeholder="Ask Mairu something..."
      />
    </Form>
  );
}
