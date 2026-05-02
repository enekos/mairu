import React, { useEffect } from "react";
import { View, Text, Pressable, StyleSheet, Modal } from "react-native";
import * as Haptics from "expo-haptics";
import { PermissionRequest } from "../state/store";

type Outcome = "allow" | "deny" | "always_allow";

type Props = {
  request: PermissionRequest;
  onResolve: (id: number | string, result: { outcome: Outcome }) => void;
};

export function PermissionModal({ request, onResolve }: Props) {
  useEffect(() => {
    Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning);
  }, [request.id]);
  const tc = (request.params as any)?.toolCall ?? {};
  return (
    <Modal
      visible
      animationType="slide"
      transparent
      onRequestClose={() => onResolve(request.id, { outcome: "deny" })}
    >
      <View style={s.sheet}>
        <Text style={s.title}>{tc.name ?? "tool"}</Text>
        <Text style={s.code}>{JSON.stringify(tc.args ?? {}, null, 2)}</Text>
        <View style={s.row}>
          <Pressable
            style={[s.btn, s.deny]}
            onPress={() => onResolve(request.id, { outcome: "deny" })}
          >
            <Text style={s.btnText}>Deny</Text>
          </Pressable>
          <Pressable
            style={s.btn}
            onPress={() => onResolve(request.id, { outcome: "allow" })}
          >
            <Text style={s.btnText}>Allow</Text>
          </Pressable>
          <Pressable
            style={[s.btn, s.always]}
            onPress={() => onResolve(request.id, { outcome: "always_allow" })}
          >
            <Text style={s.btnText}>Always</Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  );
}

const s = StyleSheet.create({
  sheet: {
    marginTop: "auto",
    backgroundColor: "white",
    padding: 16,
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
  },
  title: { fontSize: 18, fontWeight: "600", marginBottom: 8 },
  code: { fontFamily: "Menlo", fontSize: 12, marginBottom: 16, maxHeight: 240 },
  row: { flexDirection: "row", gap: 8 },
  btn: {
    flex: 1,
    padding: 14,
    borderRadius: 8,
    backgroundColor: "#2e7df6",
    alignItems: "center",
  },
  btnText: { color: "white", fontWeight: "600" },
  deny: { backgroundColor: "#c33" },
  always: { backgroundColor: "#444" },
});
