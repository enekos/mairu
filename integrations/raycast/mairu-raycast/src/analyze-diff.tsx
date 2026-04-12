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

  async function handleSubmit(values: { directory: string[] }) {
    if (!values.directory || values.directory.length === 0) {
      showToast({ style: Toast.Style.Failure, title: "Directory is required" });
      return;
    }

    const dir = values.directory[0];

    setIsLoading(true);
    try {
      const cmd = `analyze diff -o plain`;
      const stdout = await runMairuCmd(cmd, undefined, dir);
      setResult(stdout);
    } catch (error: Error | unknown) {
      showToast({
        style: Toast.Style.Failure,
        title: "Analysis failed",
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
            <Action title="New Analysis" onAction={() => setResult(null)} />
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
          <Action.SubmitForm title="Analyze Diff" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.FilePicker
        id="directory"
        title="Project Directory"
        allowMultipleSelection={false}
        canChooseFiles={false}
        canChooseDirectories={true}
      />
    </Form>
  );
}
