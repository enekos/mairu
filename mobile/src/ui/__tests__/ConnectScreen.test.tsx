import React from "react";
import { fireEvent, render, waitFor } from "@testing-library/react-native";
import { ConnectScreen } from "../ConnectScreen";
import { useStore } from "../../state/store";

jest.mock("../../api/sessions", () => ({
  listSessions: jest.fn().mockResolvedValue([]),
}));

jest.mock("@react-native-async-storage/async-storage", () => ({
  __esModule: true,
  default: {
    getItem: jest.fn().mockResolvedValue(null),
    setItem: jest.fn().mockResolvedValue(undefined),
    removeItem: jest.fn().mockResolvedValue(undefined),
  },
}));

beforeEach(() => useStore.getState().reset());

test("validates host via /sessions ping and stores it", async () => {
  const { getByPlaceholderText, getByText } = render(<ConnectScreen />);
  fireEvent.changeText(getByPlaceholderText(/host/i), "http://100.64.0.1:7777");
  fireEvent.press(getByText("Connect"));
  await waitFor(() =>
    expect(useStore.getState().host).toBe("http://100.64.0.1:7777"),
  );
});

test("rejects empty host", async () => {
  const { getByText, queryByText } = render(<ConnectScreen />);
  fireEvent.press(getByText("Connect"));
  await waitFor(() => expect(queryByText(/required/i)).toBeTruthy());
});

test("surfaces unreachable host", async () => {
  const { listSessions } = require("../../api/sessions");
  listSessions.mockRejectedValueOnce(new Error("network down"));
  const { getByPlaceholderText, getByText, findByText } = render(<ConnectScreen />);
  fireEvent.changeText(getByPlaceholderText(/host/i), "http://does-not-exist:1");
  fireEvent.press(getByText("Connect"));
  expect(await findByText(/unreachable/i)).toBeTruthy();
  expect(useStore.getState().host).toBeNull();
});
