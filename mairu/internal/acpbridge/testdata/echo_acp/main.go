//go:build acpbridgefixture

package main

import (
	"bufio"
	"encoding/json"
	"os"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	enc := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		var raw json.RawMessage = sc.Bytes()
		_ = enc.Encode(map[string]json.RawMessage{"echo": raw})
	}
}
