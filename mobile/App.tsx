import React, { useCallback, useEffect, useMemo, useState } from "react";
import { SafeAreaView, View, Text, StyleSheet } from "react-native";
import { useStore } from "./src/state/store";
import { ConnectScreen, loadStoredHost } from "./src/ui/ConnectScreen";
import { SessionPicker } from "./src/ui/SessionPicker";
import { Timeline } from "./src/ui/Timeline";
import { Composer } from "./src/ui/Composer";
import { PermissionModal } from "./src/ui/PermissionModal";
import { WSTransport } from "./src/acp/transport";
import { ACPClient } from "./src/acp/client";
import { attachSession } from "./src/state/sessionGlue";
import { Recorder } from "./src/voice/recorder";
import { ConnectionDot } from "./src/ui/ConnectionDot";

export default function App() {
  const host = useStore((s) => s.host);
  const sid = useStore((s) => s.selectedSessionId);
  const setHost = useStore((s) => s.setHost);
  const setConn = useStore((s) => s.setConnection);
  const pending = useStore((s) => s.pendingPermissions);
  const activeTurn = useStore((s) =>
    s.selectedSessionId ? !!s.activeTurnsBySession[s.selectedSessionId] : false,
  );
  const [draft, setDraft] = useState("");

  const recorder = useMemo(() => new Recorder(), []);
  useEffect(() => {
    recorder.onResult((t) => setDraft((d) => (d ? d + " " + t : t)));
  }, [recorder]);
  const startRec = useCallback(() => {
    recorder.start().catch(() => {});
  }, [recorder]);
  const stopRec = useCallback(() => {
    recorder.stop().catch(() => {});
  }, [recorder]);

  // Restore stored host on cold start.
  useEffect(() => {
    loadStoredHost().then((h) => h && setHost(h));
  }, [setHost]);

  // Build a fresh transport+client whenever host or sid changes.
  const wired = useMemo(() => {
    if (!host || !sid) return null;
    const wsUrl = host.replace(/^http/, "ws") + "/acp";
    const t = new WSTransport({ baseUrl: wsUrl, sessionId: sid });
    const c = new ACPClient(t);
    const glue = attachSession(c, sid);
    t.onState((state) => setConn(state));
    t.connect();
    return { t, c, glue };
  }, [host, sid, setConn]);

  useEffect(() => {
    return () => {
      wired?.t.disconnect();
    };
  }, [wired]);

  if (!host) {
    return (
      <SafeAreaView style={s.full}>
        <ConnectScreen />
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={s.full}>
      <View style={s.header}>
        <Text style={s.h}>mairu</Text>
        <ConnectionDot />
      </View>
      {!sid ? (
        <SessionPicker />
      ) : (
        <>
          <Timeline />
          <Composer
            active={activeTurn}
            draft={draft}
            onDraftChange={setDraft}
            onSubmit={(text) => {
              wired?.c.notify("session/prompt", { text });
              setDraft("");
            }}
            onCancel={() => wired?.c.notify("session/cancel", {})}
            onStartRecord={startRec}
            onStopRecord={stopRec}
          />
        </>
      )}
      {pending[0] && wired && (
        <PermissionModal
          request={pending[0]}
          onResolve={(id, result) => wired.glue.resolveWith(id, result)}
        />
      )}
    </SafeAreaView>
  );
}

const s = StyleSheet.create({
  full: { flex: 1 },
  header: {
    padding: 12,
    borderBottomWidth: 1,
    borderColor: "#eee",
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  h: { fontWeight: "700" },
});
