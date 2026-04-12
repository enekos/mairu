import { ActionPanel, Action, List } from "@raycast/api";
import { useState, useEffect } from "react";
import { runMairuCmd } from "./mairu-cli";

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
      } catch (error) {
        console.error(error);
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
            </ActionPanel>
          }
        />
      ))}
    </List>
  );
}
