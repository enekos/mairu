import React from "react";
import { render } from "@testing-library/react-native";
import { ConnectionDot } from "../ConnectionDot";
import { useStore } from "../../state/store";

beforeEach(() => useStore.getState().reset());

function colorFor(): string {
  const { getByTestId } = render(<ConnectionDot />);
  const dot = getByTestId("connection-dot");
  // style may be array or single object; flatten and read backgroundColor
  const flat = Array.isArray(dot.props.style)
    ? Object.assign({}, ...dot.props.style)
    : dot.props.style;
  return flat.backgroundColor as string;
}

test("idle is grey", () => {
  expect(colorFor()).toBe("#888");
});

test("connecting is yellow", () => {
  useStore.getState().setConnection("connecting");
  expect(colorFor()).toBe("#dc9b18");
});

test("open is green", () => {
  useStore.getState().setConnection("open");
  expect(colorFor()).toBe("#1f9d3a");
});

test("closed is grey", () => {
  useStore.getState().setConnection("closed");
  expect(colorFor()).toBe("#888");
});
