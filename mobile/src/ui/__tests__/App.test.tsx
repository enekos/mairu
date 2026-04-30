jest.mock("../../api/sessions", () => ({
  listSessions: jest.fn().mockResolvedValue([
    { id: "s1", agent: "mairu", started_at: 0, last_activity_at: 0, active: true },
  ]),
  createSession: jest.fn(),
}));
jest.mock("@react-native-async-storage/async-storage", () => ({
  __esModule: true,
  default: {
    getItem: jest.fn().mockResolvedValue(null),
    setItem: jest.fn(),
    removeItem: jest.fn(),
  },
}));
jest.mock("expo-haptics", () => ({
  __esModule: true,
  notificationAsync: jest.fn().mockResolvedValue(undefined),
  NotificationFeedbackType: { Warning: "warning" },
}));

import React from "react";
import { render, waitFor } from "@testing-library/react-native";
import App from "../../../App";
import { useStore } from "../../state/store";
import { FakeWebSocket } from "../../acp/testing/fakeWebSocket";

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
  useStore.getState().reset();
});

test("shows ConnectScreen when no host", () => {
  const { getByText } = render(<App />);
  expect(getByText(/Connect to mairu/i)).toBeTruthy();
});

test("shows session picker once host is set", async () => {
  useStore.getState().setHost("http://h:7777");
  const { findByText } = render(<App />);
  await findByText("s1");
});
