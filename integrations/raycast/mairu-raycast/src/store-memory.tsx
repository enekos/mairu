import { Form, ActionPanel, Action, showToast, Toast } from "@raycast/api";
import { useState } from "react";
import { runMairuCmd } from "./mairu-cli";

export default function Command({
  initialContent = "",
}: {
  initialContent?: string;
}) {
  const [isLoading, setIsLoading] = useState(false);

  async function handleSubmit(values: {
    content: string;
    category: string;
    importance: string;
  }) {
    setIsLoading(true);
    try {
      const importance = parseInt(values.importance, 10);
      const cmd = `memory store "${values.content.replace(/"/g, '\\"')}" -c "${values.category}" -i ${importance}`;
      await runMairuCmd(cmd);
      showToast({ style: Toast.Style.Success, title: "Memory Stored" });
    } catch (error: Error | unknown) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to store memory",
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
          <Action.SubmitForm title="Store Memory" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextArea
        id="content"
        title="Content"
        placeholder="Memory content..."
        defaultValue={initialContent}
      />
      <Form.TextField
        id="category"
        title="Category"
        placeholder="e.g., observation, constraint"
        defaultValue="observation"
      />
      <Form.Dropdown id="importance" title="Importance" defaultValue="5">
        {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((val) => (
          <Form.Dropdown.Item
            key={val}
            value={val.toString()}
            title={val.toString()}
          />
        ))}
      </Form.Dropdown>
    </Form>
  );
}
