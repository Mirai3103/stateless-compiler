package main

import (
	"fmt"
)

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + 1
}

func main() {
	var n int
	fmt.Scan(&n)
	fmt.Printf("%d", fibonacci(n))
}
