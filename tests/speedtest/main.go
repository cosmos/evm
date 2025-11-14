package main

import "os"

func main() {
	dir, err := os.MkdirTemp("", "speedtest-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := NewSpeedTestCommand(dir).Execute(); err != nil {
		panic(err)
	}
}
