package complex

import "fmt"

// ComplexType tests various type declarations
type ComplexType[T any] struct {
	Items []T
}

type StringMap map[string]string

// Processor is an interface
type Processor interface {
	Process(data string) error
}

// Inline closure assignment
var ProcessFunc = func(a int) int {
	return a * 2
}

// Method with pointer receiver and generics
func (c *ComplexType[T]) DoSomething(item T) {
	fmt.Println(item)
}

// Function returning a closure
func MakeClosure() func(int) int {
	return func(b int) int {
		return ProcessFunc(b) + 1
	}
}
