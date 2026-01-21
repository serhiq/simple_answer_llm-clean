package main

import (
	"fmt"
	"os"

	"simple_answer_llm/internal"
)

func main() {
	if err := internal.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
