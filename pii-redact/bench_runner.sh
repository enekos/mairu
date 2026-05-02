#!/bin/bash
set -e

# Run benchmarks and extract metrics
output=$(go test -bench=. -benchmem -count=5 ./internal/pipeline/ 2>&1)

echo "$output"

# Extract key benchmark results
text_4kb=$(echo "$output" | grep "^BenchmarkPipelineText_4KB-" | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
text_clean=$(echo "$output" | grep "^BenchmarkPipelineText_Clean-" | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
text_64kb=$(echo "$output" | grep "^BenchmarkPipelineText_64KB-" | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
cmd_simple=$(echo "$output" | grep "^BenchmarkPipelineCommand_Simple-" | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')
json_1kb=$(echo "$output" | grep "^BenchmarkJSONWalker_1KB-" | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')

allocs_4kb=$(echo "$output" | grep "^BenchmarkPipelineText_4KB-" | awk '{sum+=$5; n++} END {if(n>0) printf "%.0f", sum/n}')
bytes_4kb=$(echo "$output" | grep "^BenchmarkPipelineText_4KB-" | awk '{sum+=$7; n++} END {if(n>0) printf "%.0f", sum/n}')

# Also run the original benchmark for compatibility
text_orig=$(echo "$output" | grep "^BenchmarkPipelineText-12" | awk '{sum+=$3; n++} END {if(n>0) printf "%.0f", sum/n}')

echo ""
echo "METRIC text_4KB_ns_op=$text_4kb"
echo "METRIC text_clean_ns_op=$text_clean"
echo "METRIC text_64KB_ns_op=$text_64kb"
echo "METRIC cmd_simple_ns_op=$cmd_simple"
echo "METRIC json_1KB_ns_op=$json_1kb"
echo "METRIC text_4KB_allocs=$allocs_4kb"
echo "METRIC text_4KB_bytes=$bytes_4kb"
echo "METRIC text_orig_ns_op=$text_orig"
