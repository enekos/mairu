import React, { useState } from "react";
import { View, TextInput, Pressable, Text, StyleSheet } from "react-native";

type Props = {
  onSubmit: (text: string) => void;
  onCancel: () => void;
  active: boolean;
  onStartRecord?: () => void;
  onStopRecord?: () => void;
  draft?: string;
  onDraftChange?: (text: string) => void;
};

export function Composer({
  onSubmit,
  onCancel,
  active,
  onStartRecord,
  onStopRecord,
  draft,
  onDraftChange,
}: Props) {
  const [internal, setInternal] = useState("");
  const text = draft ?? internal;
  const setText = onDraftChange ?? setInternal;
  function send() {
    const trimmed = text.trim();
    if (!trimmed) return;
    onSubmit(text);
    setText("");
  }
  return (
    <View style={s.row}>
      <TextInput
        placeholder="Message"
        value={text}
        onChangeText={setText}
        style={s.input}
        multiline
      />
      {onStartRecord && onStopRecord && (
        <Pressable
          testID="mic"
          onPressIn={onStartRecord}
          onPressOut={onStopRecord}
          style={[s.btn, s.mic]}
        >
          <Text style={s.btnText}>🎤</Text>
        </Pressable>
      )}
      {active && (
        <Pressable onPress={onCancel} style={[s.btn, s.stop]}>
          <Text style={s.stopText}>Stop</Text>
        </Pressable>
      )}
      <Pressable onPress={send} style={s.btn}>
        <Text style={s.btnText}>Send</Text>
      </Pressable>
    </View>
  );
}

const s = StyleSheet.create({
  row: {
    flexDirection: "row",
    padding: 8,
    gap: 6,
    borderTopWidth: 1,
    borderColor: "#eee",
  },
  input: {
    flex: 1,
    borderWidth: 1,
    borderColor: "#ddd",
    borderRadius: 8,
    padding: 8,
    maxHeight: 120,
  },
  btn: {
    backgroundColor: "#2e7df6",
    paddingHorizontal: 14,
    justifyContent: "center",
    borderRadius: 8,
  },
  btnText: { color: "white", fontWeight: "600" },
  stop: { backgroundColor: "#c33" },
  stopText: { color: "white", fontWeight: "600" },
  mic: { backgroundColor: "#444" },
});
