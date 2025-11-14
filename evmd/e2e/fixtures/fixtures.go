package fixtures

import (
	"encoding/json"
	"os"
)

func loadFixture[T any](filename string) (*T, error) {
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	fixture := new(T)
	if err := json.Unmarshal(jsonData, fixture); err != nil {
		return nil, err
	}
	return fixture, nil
}
