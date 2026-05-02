#!/bin/bash
set -euo pipefail

cd /Users/enekosarasola/mairu/mairu

# Run focused agent benchmark with faster settings
output=$(go test ./internal/agent/ -run='^$' -bench='BenchmarkAgentTurn|BenchmarkAgentRunStream|BenchmarkTruncateTailLarge|BenchmarkTruncateHeadLarge|BenchmarkReadFile' -benchtime=100ms -count=3 2>&1)

# Parse results and emit METRIC lines
agent_turn=$(echo "$output" | grep 'BenchmarkAgentTurn-' | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
agent_run=$(echo "$output" | grep 'BenchmarkAgentRunStream-' | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
truncate_tail=$(echo "$output" | grep 'BenchmarkTruncateTailLarge-' | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
truncate_head=$(echo "$output" | grep 'BenchmarkTruncateHeadLarge-' | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
readfile=$(echo "$output" | grep 'BenchmarkReadFile-' | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')

echo "$output"

echo ""
echo "METRIC agent_turn_ns=$agent_turn"
echo "METRIC agent_run_ns=$agent_run"
echo "METRIC truncate_tail_ns=$truncate_tail"
echo "METRIC truncate_head_ns=$truncate_head"
echo "METRIC readfile_ns=$readfile"
