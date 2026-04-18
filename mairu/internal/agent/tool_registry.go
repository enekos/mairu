package agent

import "mairu/internal/llm"

var builtinTools = []BuiltinTool{
	&bashTool{},
	&readFileTool{},
	&writeFileTool{},
	&deleteFileTool{},
	&replaceBlockTool{},
	&multiEditTool{},
	&findFilesTool{},
	&searchCodebaseTool{},
	&fetchURLTool{},
	&scrapeURLTool{},
	&searchWebTool{},
	&delegateTaskTool{},
	&reviewWorkTool{},
	&browserContextTool{},
}

var builtinToolsByName map[string]BuiltinTool

func init() {
	builtinToolsByName = make(map[string]BuiltinTool, len(builtinTools))
	for _, bt := range builtinTools {
		builtinToolsByName[bt.Definition().Name] = bt
	}
}

func builtinToolSchemas() []llm.Tool {
	schemas := make([]llm.Tool, len(builtinTools))
	for i, bt := range builtinTools {
		schemas[i] = bt.Definition()
	}
	return schemas
}

func findBuiltinTool(name string) BuiltinTool {
	return builtinToolsByName[name]
}
