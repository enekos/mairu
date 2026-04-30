import React, { useState } from "react";
import { View, Text, TextInput, Pressable, StyleSheet } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { useStore } from "../state/store";
import { listSessions } from "../api/sessions";

const KEY = "mairu.host";

export function ConnectScreen() {
  const [host, setHostText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const setHost = useStore((s) => s.setHost);

  async function onConnect() {
    if (!host.trim()) {
      setError("host is required");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await listSessions(host);
      await AsyncStorage.setItem(KEY, host);
      setHost(host);
    } catch (e: any) {
      setError(`unreachable: ${e?.message ?? e}`);
    } finally {
      setBusy(false);
    }
  }

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Connect to mairu acp-bridge</Text>
      <TextInput
        placeholder="host (http://100.x.x.x:7777)"
        autoCapitalize="none"
        autoCorrect={false}
        value={host}
        onChangeText={setHostText}
        style={styles.input}
      />
      {error && <Text style={styles.err}>{error}</Text>}
      <Pressable disabled={busy} onPress={onConnect} style={styles.btn}>
        <Text style={styles.btnText}>{busy ? "Connecting…" : "Connect"}</Text>
      </Pressable>
    </View>
  );
}

export async function loadStoredHost(): Promise<string | null> {
  return AsyncStorage.getItem(KEY);
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", padding: 24, gap: 12 },
  title: { fontSize: 18, fontWeight: "600" },
  input: { borderWidth: 1, borderColor: "#444", borderRadius: 8, padding: 12 },
  err: { color: "#c33" },
  btn: { backgroundColor: "#222", padding: 14, borderRadius: 8, alignItems: "center" },
  btnText: { color: "white", fontWeight: "600" },
});
