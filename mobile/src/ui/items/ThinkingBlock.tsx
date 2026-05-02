import React, { useState } from "react";
import { Pressable, Text, StyleSheet } from "react-native";

export function ThinkingBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <Pressable onPress={() => setOpen((o) => !o)} style={s.b}>
      <Text style={s.label}>Thinking…</Text>
      {open && <Text style={s.text}>{text}</Text>}
    </Pressable>
  );
}

const s = StyleSheet.create({
  b: { padding: 8, margin: 6, opacity: 0.6 },
  label: { fontStyle: "italic" },
  text: { fontSize: 12, marginTop: 4 },
});
