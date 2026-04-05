package eval

import "fmt"

func TestSlice() {
	d := EvalDataset{Cases: []Case{{ID: "1"}}}
	mod(d)
	fmt.Println(d.Cases[0].Got)
}
func mod(d EvalDataset) {
	d.Cases[0].Got = []RetrievalResult{{ID: "2"}}
}
