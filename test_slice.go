package main

import "fmt"

type Case struct{ Got []string }
type EvalDataset struct{ Cases []Case }

func main() {
	d := EvalDataset{Cases: []Case{{}}}
	mod(d)
	fmt.Println(d.Cases[0].Got)
}
func mod(d EvalDataset) {
	d.Cases[0].Got = []string{"2"}
}
