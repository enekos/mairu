#!/bin/bash
grep -n -A 10 "msg.Type == \"done\"" mairu/internal/tui/update.go
