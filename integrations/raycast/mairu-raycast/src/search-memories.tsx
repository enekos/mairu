import {
  ActionPanel,
  Action,
  List,
  Icon,
  showToast,
  Toast,
} from "@raycast/api";
import { useState, useEffect } from "react";
import { runMairuCmd } from "./mairu-cli";
import StoreMemory from "./store-memory";

interface Memory {
  id: string;
  content: string;
  _score: number;
}

interface MairuResponse {
  memories?: Memory[];
}

export default function Command() {
  const [searchText, setSearchText] = useState("");
  const [memories, setMemories] = useState<Memory[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    async function search() {
      if (!searchText) {
        setMemories([]);
        return;
      }
      setIsLoading(true);
      try {
        const stdout = await runMairuCmd(
          `memory search "${searchText.replace(/"/g, '\\"')}"`,
        );
        const data: MairuResponse = JSON.parse(stdout);
        setMemories(data.memories || []);
      } catch (error: Error | unknown) {
        await showToast({
          style: Toast.Style.Failure,
          title: "Search failed",
          message: (error as Error).message,
        });
      } finally {
        setIsLoading(false);
      }
    }

    const timeoutId = setTimeout(search, 300);
    return () => clearTimeout(timeoutId);
  }, [searchText]);

  return (
    <List
      isLoading={isLoading}
      onSearchTextChange={setSearchText}
      searchBarPlaceholder="Search memories..."
      throttle
    >
      {memories.length === 0 && searchText.length > 0 && !isLoading && (
        <List.EmptyView
          title="No memories found"
          description="Press Enter to store this as a new memory"
          actions={
            <ActionPanel>
              <Action.Push
                title="Store New Memory"
                icon={Icon.Plus}
                target={<StoreMemory initialContent={searchText} />}
              />
            </ActionPanel>
          }
        />
      )}
      {memories.map((memory) => (
        <List.Item
          key={memory.id}
          title={memory.content}
          subtitle={`Score: ${memory._score.toFixed(2)}`}
          actions={
            <ActionPanel>
              <Action.CopyToClipboard
                title="Copy Content"
                content={memory.content}
              />
              <Action.CopyToClipboard title="Copy Id" content={memory.id} />
              <Action.Push
                title="Store New Memory"
                icon={Icon.Plus}
                target={<StoreMemory initialContent={searchText} />}
                shortcut={{ modifiers: ["cmd"], key: "n" }}
              />
            </ActionPanel>
          }
        />
      ))}
    </List>
  );
}
