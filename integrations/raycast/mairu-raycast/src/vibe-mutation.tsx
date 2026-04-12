import { Form, ActionPanel, Action, showToast, Toast } from "@raycast/api";
import { useState } from "react";
import { runMairuCmd } from "./mairu-cli";

export default function Command() {
  const [isLoading, setIsLoading] = useState(false);

  async function handleSubmit(values: { instructions: string }) {
    if (!values.instructions) {
      showToast({
        style: Toast.Style.Failure,
        title: "Instructions are required",
      });
      return;
    }

    setIsLoading(true);
    try {
      const cmd = `vibe-mutation "${values.instructions.replace(/"/g, '\\"')}" -y`;
      await runMairuCmd(cmd);
      showToast({
        style: Toast.Style.Success,
        title: "Context Updated Successfully",
      });
    } catch (error: Error | unknown) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to update context",
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
          <Action.SubmitForm title="Apply Mutation" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextArea
        id="instructions"
        title="Instructions"
        placeholder="e.g., Remember that we switched to gRPC for internal service calls."
      />
    </Form>
  );
}
