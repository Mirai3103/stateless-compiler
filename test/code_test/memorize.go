package main

import (
	"fmt"
)

var memo = map[int]int{}

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	if val, ok := memo[n]; ok {
		return val
	}
	memo[n] = fibonacci(n-1) + fibonacci(n-2)
	return memo[n]
}

func main() {
	var n int
	fmt.Scan(&n)
	fmt.Printf("%d", fibonacci(n))
}
