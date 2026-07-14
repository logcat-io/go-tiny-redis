package main

import "fmt"

func main() {
	m := make(map[string]string)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < 1_000_000; i++ {
			m["k"] = "goroutine-1"
		}
	}()
	for i := 0; i < 1_000_000; i++ {
		m["k"] = "goroutine-2"
	}

	<-done
	fmt.Println("fatal 발생 실패")
}
