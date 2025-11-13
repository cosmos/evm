package main

import "os"

func main() {
	dir, err := os.MkdirTemp("", "mytmp-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := NewSpeedTestCommand(dir).Execute(); err != nil {
		panic(err)
	}
}
