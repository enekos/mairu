package llm

import (
	"github.com/google/generative-ai-go/genai"
)

func (g *GeminiProvider) SetupTools() {
	tools := []*genai.FunctionDeclaration{
		{
			Name:        "replace_block",
			Description: "Safely apply a Search-and-Replace block edit to a file. You must provide the EXACT existing code block you want to replace, including all whitespace. This is much safer and more reliable than multi_edit.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"file_path": {
						Type:        genai.TypeString,
						Description: "The relative path to the file.",
					},
					"old_code": {
						Type:        genai.TypeString,
						Description: "The exact existing code block to be replaced. Must match exactly, including indentation.",
					},
					"new_code": {
						Type:        genai.TypeString,
						Description: "The new code block to insert in its place.",
					},
				},
				Required: []string{"file_path", "old_code", "new_code"},
			},
		},
		{
			Name:        "multi_edit",
			Description: "Apply a block replacement to a specific file.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"file_path": {
						Type:        genai.TypeString,
						Description: "The relative path to the file.",
					},
					"start_line": {
						Type:        genai.TypeInteger,
						Description: "The 1-indexed starting line to replace.",
					},
					"end_line": {
						Type:        genai.TypeInteger,
						Description: "The 1-indexed ending line to replace.",
					},
					"content": {
						Type:        genai.TypeString,
						Description: "The new content to insert in place of those lines.",
					},
				},
				Required: []string{"file_path", "start_line", "end_line", "content"},
			},
		},
		{
			Name:        "bash",
			Description: "Execute a bash command in the project root directory. Use this to run tests, linters, or explore the file system.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"command": {
						Type:        genai.TypeString,
						Description: "The bash command to execute.",
					},
					"timeout_ms": {
						Type:        genai.TypeInteger,
						Description: "Optional timeout in milliseconds (default 30000).",
					},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "read_file",
			Description: "Read the contents of a file. Supports reading specific sections using offset and limit. Output is truncated to 2000 lines by default. Use offset/limit for large files.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"file_path": {
						Type:        genai.TypeString,
						Description: "The relative path to the file.",
					},
					"offset": {
						Type:        genai.TypeInteger,
						Description: "The line number to start reading from (1-indexed). Defaults to 1.",
					},
					"limit": {
						Type:        genai.TypeInteger,
						Description: "Maximum number of lines to read. Defaults to 2000.",
					},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file, overwriting it completely. If editing an existing file, prefer multi_edit.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"file_path": {
						Type:        genai.TypeString,
						Description: "The relative path to the file.",
					},
					"content": {
						Type:        genai.TypeString,
						Description: "The entire new content of the file.",
					},
				},
				Required: []string{"file_path", "content"},
			},
		},
		{
			Name:        "find_files",
			Description: "Find files by glob pattern.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"pattern": {
						Type:        genai.TypeString,
						Description: "The glob pattern (e.g., src/**/*.ts).",
					},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name:        "search_codebase",
			Description: "Search the codebase by text/regex query or by symbol name (surgical read).",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"query": {
						Type:        genai.TypeString,
						Description: "Text or regex to search in files.",
					},
					"symbol_name": {
						Type:        genai.TypeString,
						Description: "Exact symbol name to look up (function, method, class).",
					},
				},
				Required: []string{},
			},
		},
		{
			Name:        "review_work",
			Description: "Before finishing a task, use this tool to review the work done against the requirements, and self-critique it for potential flaws or missed edge cases. This ensures better accuracy and reliability.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"summary": {
						Type:        genai.TypeString,
						Description: "A summary of the changes made and how they resolve the task.",
					},
					"critique": {
						Type:        genai.TypeString,
						Description: "A self-critique identifying any edge cases, potential failures, or unaddressed requirements.",
					},
				},
				Required: []string{"summary", "critique"},
			},
		},
		{
			Name:        "delegate_task",
			Description: "Delegate a complex sub-task to another AI agent. Useful for researching or exploring while you focus on the main task.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"task_description": {
						Type:        genai.TypeString,
						Description: "A highly detailed prompt describing what the sub-agent should do.",
					},
				},
				Required: []string{"task_description"},
			},
		},
		{
			Name:        "scrape_url",
			Description: "Scrape a web page and extract structured information based on a prompt. Use this when you need specific data extracted intelligently from a website.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"url": {
						Type:        genai.TypeString,
						Description: "The full URL to scrape (e.g., https://example.com).",
					},
					"prompt": {
						Type:        genai.TypeString,
						Description: "The instructions on what information to extract from the page.",
					},
				},
				Required: []string{"url", "prompt"},
			},
		},
		{
			Name:        "search_web",
			Description: "Search the web for a query and extract structured information from the top results based on a prompt.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"query": {
						Type:        genai.TypeString,
						Description: "The search query to look up on the web.",
					},
					"prompt": {
						Type:        genai.TypeString,
						Description: "The instructions on what information to extract from the search results.",
					},
				},
				Required: []string{"query", "prompt"},
			},
		},
		{
			Name:        "fetch_url",
			Description: "Fetch the text content of a web page by URL. Useful for reading documentation or external resources.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"url": {
						Type:        genai.TypeString,
						Description: "The full URL to fetch (e.g., https://example.com).",
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "delete_file",
			Description: "Delete a file or directory.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The relative path to the file or directory.",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "browser_context",
			Description: "Get real-time browser context from the Mairu browser extension.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"command": {
						Type:        genai.TypeString,
						Description: "The command to run: current, history, search, or session.",
					},
					"query": {
						Type:        genai.TypeString,
						Description: "The search query (only for 'search' command).",
					},
					"limit": {
						Type:        genai.TypeInteger,
						Description: "The limit for search results (only for 'search' command).",
					},
				},
				Required: []string{"command"},
			},
		},
	}

	g.model.Tools = []*genai.Tool{
		{FunctionDeclarations: tools},
	}
}

func (g *GeminiProvider) RegisterDynamicTools(tools []*genai.FunctionDeclaration) {
	if len(tools) == 0 {
		return
	}
	if len(g.model.Tools) == 0 {
		g.model.Tools = []*genai.Tool{{FunctionDeclarations: tools}}
		return
	}
	g.model.Tools[0].FunctionDeclarations = append(g.model.Tools[0].FunctionDeclarations, tools...)
}
