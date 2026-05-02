import React, { useCallback, useEffect, useState } from "react";
import { View, Text, Pressable, FlatList, StyleSheet, Modal } from "react-native";
import { listSessions, createSession, SessionInfo } from "../api/sessions";
import { useStore } from "../state/store";

const AGENTS = ["mairu", "claude-code", "gemini"] as const;

export function SessionPicker() {
  const host = useStore((s) => s.host);
  const select = useStore((s) => s.selectSession);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [creating, setCreating] = useState(false);

  const refresh = useCallback(async () => {
    if (!host) return;
    setSessions(await listSessions(host));
  }, [host]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function onCreate(agent: string) {
    if (!host) return;
    const id = await createSession(host, agent);
    setCreating(false);
    select(id);
    refresh();
  }

  return (
    <View style={styles.box}>
      <FlatList
        data={sessions}
        keyExtractor={(s) => s.id}
        ListHeaderComponent={
          <Pressable onPress={() => setCreating(true)} style={styles.row}>
            <Text style={styles.plus}>+ New session</Text>
          </Pressable>
        }
        renderItem={({ item }) => (
          <Pressable onPress={() => select(item.id)} style={styles.row}>
            <Text>{item.id}</Text>
            <Text style={styles.dim}>
              {item.agent}
              {item.active ? " · active" : ""}
            </Text>
          </Pressable>
        )}
      />
      <Modal
        visible={creating}
        transparent
        animationType="slide"
        onRequestClose={() => setCreating(false)}
      >
        <View style={styles.sheet}>
          {AGENTS.map((a) => (
            <Pressable key={a} onPress={() => onCreate(a)} style={styles.row}>
              <Text>{a}</Text>
            </Pressable>
          ))}
          <Pressable onPress={() => setCreating(false)} style={styles.row}>
            <Text style={styles.dim}>Cancel</Text>
          </Pressable>
        </View>
      </Modal>
    </View>
  );
}

const styles = StyleSheet.create({
  box: { padding: 12 },
  row: { paddingVertical: 12, borderBottomWidth: 1, borderColor: "#eee" },
  plus: { fontWeight: "600" },
  dim: { color: "#888", fontSize: 12 },
  sheet: {
    marginTop: "auto",
    backgroundColor: "white",
    padding: 16,
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
  },
});
