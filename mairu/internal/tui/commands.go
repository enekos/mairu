package tui

type SlashCommand struct {
	Name        string
	Description string
}

var allSlashCommands = []SlashCommand{
	{"/help", "Show this help message"},
	{"/clear", "Clear the terminal screen"},
	{"/copy", "Copy last response to clipboard"},
	{"/models", "Open model selector"},
	{"/model", "Switch to a specific model"},
	{"/sessions", "Open session selector"},
	{"/session", "Load a specific session"},
	{"/memory", "Inspect memory: list|show|search|store|delete|help"},
	{"/memory list", "List recent memories"},
	{"/memory show", "Show full memory by id"},
	{"/memory search", "Search memories"},
	{"/memory store", "Store a new memory"},
	{"/memory delete", "Delete a memory by id"},
	{"/node search", "Search contextfs nodes"},
	{"/node read", "Read contextfs nodes"},
	{"/node ls", "List contextfs node children"},
	{"/node store", "Store/update a contextfs node"},
	{"/node write", "Write/update a contextfs node"},
	{"/remember", "Run contextfs vibe-mutation"},
	{"/save", "Save the current session"},
	{"/fork", "Fork the current session to a new name"},
	{"/reset", "Start a fresh session"},
	{"/new", "Start a fresh session"},
	{"/compact", "Summarize history to save tokens"},
	{"/squash", "Summarize history to save tokens"},
	{"/export", "Export conversation to a file"},
	{"/graph", "Interactive Context Graph Explorer"},
	{"/data", "Interactive Workspace Data Explorer (Nodes, Memories, Skills)"},
	{"/explore", "Toggle explore sidebar"},
	{"/logs", "Toggle internal logs sidebar"},
	{"/agent", "Focus agent pane"},
	{"/nvim", "Open Neovim pane"},
	{"/lazygit", "Open LazyGit pane"},
	{"/pane", "Switch pane: agent|nvim|lazygit"},
	{"/jump", "Jump to message number n"},
	{"/approve", "Approve pending agent action"},
	{"/deny", "Deny pending agent action"},
	{"/council", "Council mode control: /council on|off|status"},
	{"/exit", "Exit Mairu"},
	{"/quit", "Exit Mairu"},
}
