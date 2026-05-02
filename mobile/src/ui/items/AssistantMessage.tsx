import React from "react";
import { View, StyleSheet } from "react-native";
import Markdown from "react-native-markdown-display";

export function AssistantMessage({ text }: { text: string }) {
  return (
    <View style={s.b}>
      <Markdown>{text}</Markdown>
    </View>
  );
}

const s = StyleSheet.create({
  b: { padding: 8, margin: 6 },
});
