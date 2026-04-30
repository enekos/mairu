import React from "react";
import { fireEvent, render, waitFor } from "@testing-library/react-native";
import { SessionPicker } from "../SessionPicker";
import { useStore } from "../../state/store";

jest.mock("../../api/sessions", () => ({
  listSessions: jest.fn().mockResolvedValue([
    { id: "s1", agent: "mairu", started_at: 0, last_activity_at: 0, active: true },
    { id: "s2", agent: "claude-code", started_at: 0, last_activity_at: 0, active: false },
  ]),
  createSession: jest.fn().mockResolvedValue("s3"),
}));

beforeEach(() => {
  useStore.getState().reset();
  useStore.getState().setHost("http://h:7777");
});

test("lists sessions and selects one", async () => {
  const { findByText } = render(<SessionPicker />);
  const row = await findByText("s1");
  fireEvent.press(row);
  expect(useStore.getState().selectedSessionId).toBe("s1");
});

test("creates new session", async () => {
  const { findByText, getByText } = render(<SessionPicker />);
  const trigger = await findByText(/\+ New session/);
  fireEvent.press(trigger);
  fireEvent.press(getByText("mairu"));
  await waitFor(() =>
    expect(useStore.getState().selectedSessionId).toBe("s3"),
  );
});
