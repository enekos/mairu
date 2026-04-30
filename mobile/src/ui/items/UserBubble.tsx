import React from "react";
import { View, Text, StyleSheet } from "react-native";

export function UserBubble({ text }: { text: string }) {
  return (
    <View style={s.b}>
      <Text style={s.t}>{text}</Text>
    </View>
  );
}

const s = StyleSheet.create({
  b: {
    alignSelf: "flex-end",
    backgroundColor: "#2e7df6",
    padding: 10,
    borderRadius: 12,
    margin: 6,
    maxWidth: "80%",
  },
  t: { color: "white" },
});
