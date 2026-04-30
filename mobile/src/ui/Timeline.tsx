import React from "react";
import { FlatList } from "react-native";
import { useStore, TimelineEvent } from "../state/store";
import { UserBubble } from "./items/UserBubble";
import { AssistantMessage } from "./items/AssistantMessage";
import { ToolCallCard } from "./items/ToolCallCard";
import { ThinkingBlock } from "./items/ThinkingBlock";

export function Timeline() {
  const sid = useStore((s) => s.selectedSessionId);
  const events = useStore((s) => (sid ? s.eventsBySession[sid] ?? [] : []));
  return (
    <FlatList<TimelineEvent>
      data={events}
      keyExtractor={(_, i) => String(i)}
      renderItem={({ item }) => {
        switch (item.kind) {
          case "user":
            return <UserBubble text={item.text ?? ""} />;
          case "assistant":
            return <AssistantMessage text={item.text ?? ""} />;
          case "tool":
            return (
              <ToolCallCard
                name={item.toolName ?? "tool"}
                args={item.toolArgs}
                result={item.toolResult}
              />
            );
          case "thinking":
            return <ThinkingBlock text={item.text ?? ""} />;
          default:
            return null;
        }
      }}
    />
  );
}
