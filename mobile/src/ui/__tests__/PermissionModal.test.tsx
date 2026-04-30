jest.mock("expo-haptics", () => ({
  __esModule: true,
  notificationAsync: jest.fn().mockResolvedValue(undefined),
  NotificationFeedbackType: { Warning: "warning" },
}));

import React from "react";
import { fireEvent, render } from "@testing-library/react-native";
import { PermissionModal } from "../PermissionModal";

const baseReq = {
  id: 1,
  sessionId: "s",
  method: "session/request_permission",
  params: { toolCall: { name: "shell", args: { cmd: "ls" } } },
};

test("renders tool name + args, fires onResolve with allow", () => {
  const onResolve = jest.fn();
  const { getByText } = render(<PermissionModal request={baseReq} onResolve={onResolve} />);
  expect(getByText("shell")).toBeTruthy();
  fireEvent.press(getByText("Allow"));
  expect(onResolve).toHaveBeenCalledWith(1, { outcome: "allow" });
});

test("Deny returns deny", () => {
  const onResolve = jest.fn();
  const { getByText } = render(<PermissionModal request={baseReq} onResolve={onResolve} />);
  fireEvent.press(getByText("Deny"));
  expect(onResolve).toHaveBeenCalledWith(1, { outcome: "deny" });
});

test("Always returns always_allow", () => {
  const onResolve = jest.fn();
  const { getByText } = render(<PermissionModal request={baseReq} onResolve={onResolve} />);
  fireEvent.press(getByText("Always"));
  expect(onResolve).toHaveBeenCalledWith(1, { outcome: "always_allow" });
});
