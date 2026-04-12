import { ActionPanel, Action, List } from "@raycast/api";
import { useState, useEffect } from "react";
import { runMairuCmd } from "./mairu-cli";

interface ContextNode {
  uri: string;
  name: string;
  abstract: string;
  _score: number;
}

interface MairuResponse {
  contextNodes?: ContextNode[];
}

export default function Command() {
  const [searchText, setSearchText] = useState("");
  const [nodes, setNodes] = useState<ContextNode[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    async function search() {
      if (!searchText) {
        setNodes([]);
        return;
      }
      setIsLoading(true);
      try {
        const stdout = await runMairuCmd(
          `node search "${searchText.replace(/"/g, '\\"')}"`,
        );
        const data: MairuResponse = JSON.parse(stdout);
        setNodes(data.contextNodes || []);
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
      searchBarPlaceholder="Search context nodes..."
      throttle
    >
      {nodes.map((node) => (
        <List.Item
          key={node.uri}
          title={node.name}
          subtitle={
            node.abstract.substring(0, 80) +
            (node.abstract.length > 80 ? "..." : "")
          }
          accessories={[{ text: `Score: ${node._score.toFixed(2)}` }]}
          actions={
            <ActionPanel>
              <Action.CopyToClipboard title="Copy Uri" content={node.uri} />
              <Action.CopyToClipboard
                title="Copy Abstract"
                content={node.abstract}
              />
            </ActionPanel>
          }
        />
      ))}
    </List>
  );
}
