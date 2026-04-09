#!/bin/bash
sed -i '' 's/make(chan AgentEvent)/make(chan AgentEvent, 100)/g' mairu/internal/agent/agent.go
