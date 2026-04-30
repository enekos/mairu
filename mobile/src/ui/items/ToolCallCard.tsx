import React, { useState } from "react";
import { Pressable, Text, StyleSheet } from "react-native";

type Props = { name: string; args: unknown; result?: unknown };

export function ToolCallCard({ name, args, result }: Props) {
  const [open, setOpen] = useState(false);
  return (
    <Pressable onPress={() => setOpen((o) => !o)} style={s.card}>
      <Text style={s.title}>{name}</Text>
      {open && <Text style={s.code}>{JSON.stringify(args, null, 2)}</Text>}
      {open && result !== undefined && (
        <Text style={s.code}>
          {typeof result === "string" ? result : JSON.stringify(result, null, 2)}
        </Text>
      )}
    </Pressable>
  );
}

const s = StyleSheet.create({
  card: {
    borderWidth: 1,
    borderColor: "#ccc",
    borderRadius: 8,
    padding: 10,
    margin: 6,
  },
  title: { fontWeight: "600" },
  code: { fontFamily: "Menlo", marginTop: 6, fontSize: 12 },
});
