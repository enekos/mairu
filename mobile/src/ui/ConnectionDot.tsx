import React from "react";
import { View, StyleSheet } from "react-native";
import { useStore, ConnectionState } from "../state/store";

const COLORS: Record<ConnectionState, string> = {
  open: "#1f9d3a",
  connecting: "#dc9b18",
  closed: "#888",
  idle: "#888",
};

export function ConnectionDot() {
  const c = useStore((s) => s.connection);
  return (
    <View
      testID="connection-dot"
      accessibilityLabel={`connection ${c}`}
      style={[styles.dot, { backgroundColor: COLORS[c] }]}
    />
  );
}

const styles = StyleSheet.create({
  dot: { width: 10, height: 10, borderRadius: 5 },
});
