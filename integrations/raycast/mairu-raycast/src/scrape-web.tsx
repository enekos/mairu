import { Form, ActionPanel, Action, showToast, Toast } from "@raycast/api";
import { useState } from "react";
import { runMairuCmd } from "./mairu-cli";

export default function Command({ initialUrl = "" }: { initialUrl?: string }) {
  const [isLoading, setIsLoading] = useState(false);

  async function handleSubmit(values: { url: string; maxDepth: string }) {
    if (!values.url) {
      showToast({ style: Toast.Style.Failure, title: "URL is required" });
      return;
    }

    setIsLoading(true);
    try {
      const depth = values.maxDepth
        ? `--max-depth ${parseInt(values.maxDepth, 10)}`
        : "";
      const cmd = `scrape web "${values.url}" ${depth}`;
      await runMairuCmd(cmd);
      showToast({
        style: Toast.Style.Success,
        title: "Webpage Scraped and Stored",
      });
    } catch (error: Error | unknown) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to scrape web",
        message: (error as Error).message,
      });
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <Form
      isLoading={isLoading}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Scrape Web" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="url"
        title="URL"
        placeholder="https://example.com"
        defaultValue={initialUrl}
      />
      <Form.TextField
        id="maxDepth"
        title="Max Depth"
        placeholder="e.g., 3"
        defaultValue="1"
      />
    </Form>
  );
}
