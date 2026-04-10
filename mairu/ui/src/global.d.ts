interface Window {
  go?: {
    desktop?: {
      App?: {
        ListMemories(project: string, limit: number): Promise<any>;
        CreateMemory(input: any): Promise<any>;
        UpdateMemory(input: any): Promise<any>;
        DeleteMemory(id: string): Promise<any>;
        ApplyMemoryFeedback(id: string, reward: number): Promise<any>;
        
        ListSkills(project: string, limit: number): Promise<any>;
        CreateSkill(input: any): Promise<any>;
        UpdateSkill(input: any): Promise<any>;
        DeleteSkill(id: string): Promise<any>;

        ListContextNodes(project: string, parentURI: string | null, limit: number): Promise<any>;
        CreateContextNode(input: any): Promise<any>;
        UpdateContextNode(input: any): Promise<any>;
        DeleteContextNode(uri: string): Promise<any>;

        Search(opts: any): Promise<any>;
        Dashboard(limit: number, project: string): Promise<any>;
        Health(): Promise<any>;
        ClusterStats(): Promise<any>;

        VibeQuery(prompt: string, project: string, topK: number): Promise<any>;
        VibeMutationPlan(prompt: string, project: string, topK: number): Promise<any>;
        VibeMutationExecute(operations: any[], project: string): Promise<any>;

        ListModerationQueue(limit: number): Promise<any>;
        ReviewModeration(input: any): Promise<any>;

        ListSessions(): Promise<string[]>;
        CreateSession(name: string): Promise<void>;
        LoadSessionHistory(session: string): Promise<any>;
        SendMessage(session: string, text: string): void;
      };
    };
  };
  runtime?: {
    EventsOn(eventName: string, callback: (...args: any[]) => void): void;
    EventsOff(eventName: string, ...callbacks: any[]): void;
  };
}

