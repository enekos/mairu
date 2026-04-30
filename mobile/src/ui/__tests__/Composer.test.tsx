import React from "react";
import { fireEvent, render } from "@testing-library/react-native";
import { Composer } from "../Composer";

test("Send fires onSubmit with text and clears input", () => {
  const onSubmit = jest.fn();
  const { getByPlaceholderText, getByText } = render(
    <Composer onSubmit={onSubmit} onCancel={() => {}} active={false} />,
  );
  const input = getByPlaceholderText(/message/i);
  fireEvent.changeText(input, "hi");
  fireEvent.press(getByText("Send"));
  expect(onSubmit).toHaveBeenCalledWith("hi");
});

test("Send is a no-op for whitespace-only input", () => {
  const onSubmit = jest.fn();
  const { getByText, getByPlaceholderText } = render(
    <Composer onSubmit={onSubmit} onCancel={() => {}} active={false} />,
  );
  fireEvent.changeText(getByPlaceholderText(/message/i), "   ");
  fireEvent.press(getByText("Send"));
  expect(onSubmit).not.toHaveBeenCalled();
});

test("Stop button visible only when active", () => {
  const { queryByText, rerender } = render(
    <Composer onSubmit={() => {}} onCancel={() => {}} active={false} />,
  );
  expect(queryByText("Stop")).toBeNull();
  rerender(<Composer onSubmit={() => {}} onCancel={() => {}} active />);
  expect(queryByText("Stop")).toBeTruthy();
});

test("Stop press fires onCancel", () => {
  const onCancel = jest.fn();
  const { getByText } = render(
    <Composer onSubmit={() => {}} onCancel={onCancel} active />,
  );
  fireEvent.press(getByText("Stop"));
  expect(onCancel).toHaveBeenCalled();
});
