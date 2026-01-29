package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("=== ТЕСТ: Простейший запуск ===")
	fmt.Println("PID:", os.Getpid())
	fmt.Println("Working directory:", func() string {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
		return "error"
	}())
	fmt.Println("=== ТЕСТ ЗАВЕРШЕН ===")
}
