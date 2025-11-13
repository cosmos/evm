package main

func main() {
	if err := NewSpeedTestCommand().Execute(); err != nil {
		panic(err)
	}
}
