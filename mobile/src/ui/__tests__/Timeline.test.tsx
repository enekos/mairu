import React from "react";
import { fireEvent, render } from "@testing-library/react-native";
import { Timeline } from "../Timeline";
import { useStore } from "../../state/store";

beforeEach(() => {
  useStore.getState().reset();
  useStore.getState().selectSession("s1");
  useStore.getState().appendEvent("s1", { kind: "user", text: "hello" });
  useStore.getState().appendEvent("s1", { kind: "assistant", text: "**hi** there" });
  useStore.getState().appendEvent("s1", {
    kind: "tool",
    toolName: "shell",
    toolArgs: { cmd: "ls" },
    toolResult: "a\nb",
  });
  useStore.getState().appendEvent("s1", { kind: "thinking", text: "pondering" });
});

test("renders user, assistant, tool, thinking", () => {
  const { getByText, queryByText, getAllByText } = render(<Timeline />);
  expect(getByText("hello")).toBeTruthy();
  // Markdown splits "**hi** there" across multiple Text nodes; just assert
  // at least one fragment renders.
  expect(getAllByText(/hi/i).length).toBeGreaterThan(0);
  expect(getByText("shell")).toBeTruthy();
  expect(queryByText(/Thinking/)).toBeTruthy();
});

test("tool card expands to show args + result on tap", () => {
  const { getByText, queryByText } = render(<Timeline />);
  expect(queryByText(/cmd/)).toBeNull();
  fireEvent.press(getByText("shell"));
  expect(getByText(/cmd/)).toBeTruthy();
});
