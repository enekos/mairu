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

func builtinToolSchemas() []llm.Tool {
	schemas := make([]llm.Tool, len(builtinTools))
	for i, bt := range builtinTools {
		schemas[i] = bt.Definition()
	}
	return schemas
}

func findBuiltinTool(name string) BuiltinTool {
	for _, bt := range builtinTools {
		if bt.Definition().Name == name {
			return bt
		}
	}
	return nil
}
