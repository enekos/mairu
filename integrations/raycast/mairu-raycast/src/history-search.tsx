import { ActionPanel, Action, List } from "@raycast/api";
import { useState, useEffect } from "react";
import { runMairuCmd } from "./mairu-cli";

interface HistoryItem {
  id: string;
  command: string;
  output: string;
  timestamp: string;
  _score: number;
}

interface MairuResponse {
  history?: HistoryItem[];
}

export default function Command() {
  const [searchText, setSearchText] = useState("");
  const [historyItems, setHistoryItems] = useState<HistoryItem[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    async function search() {
      if (!searchText) {
        setHistoryItems([]);
        return;
      }
      setIsLoading(true);
      try {
        const stdout = await runMairuCmd(
          `history search "${searchText.replace(/"/g, '\\"')}"`,
        );
        const data: MairuResponse = JSON.parse(stdout);
        setHistoryItems(data.history || []);
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
      searchBarPlaceholder="Search bash history..."
      throttle
    >
      {historyItems.map((item) => (
        <List.Item
          key={item.id}
          title={item.command}
          subtitle={
            item.output
              ? item.output.substring(0, 80) +
                (item.output.length > 80 ? "..." : "")
              : "No output"
          }
          accessories={[{ text: `Score: ${item._score.toFixed(2)}` }]}
          actions={
            <ActionPanel>
              <Action.CopyToClipboard
                title="Copy Command"
                content={item.command}
              />
              <Action.CopyToClipboard
                title="Copy Output"
                content={item.output}
              />
            </ActionPanel>
          }
        />
      ))}
    </List>
  );
}
