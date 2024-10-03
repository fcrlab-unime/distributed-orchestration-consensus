// Eli Bendersky [https://eli.thegreenplace.net]
// This code is in the public domain.
package storage

import (
	"encoding/json"
	"os"
	"sync"
)

// Storage is an interface implemented by stable storage providers.
type Storage interface {
	Set(value map[string]interface{}, toWrite bool)
}

// MapStorage is a simple in-memory implementation of Storage for testing.
type MapStorage struct {
	mu sync.Mutex
	m  map[string]map[string]interface{}
	f  string
}

func NewMapStorage() *MapStorage {
	m := make(map[string]map[string]interface{})
	ms := &MapStorage{
		m: m,
		f: os.Getenv("LOG_PATH"),
		mu: sync.Mutex{},
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()
	jsonRead, err := os.ReadFile(ms.f)

	if err != nil {
		fd, _ := os.Create(ms.f)
		defer fd.Close()
		jsonWrite, _ := json.MarshalIndent(ms.m, "", "  ")
		fd.Write(jsonWrite)
	} else {
		json.Unmarshal(jsonRead, &ms.m)
	}

	return ms

}

func (ms *MapStorage) Set(value map[string]interface{}, toWrite bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	id := value["Id"].(string)
	delete(value, "Id")

	if ms.m[id] == nil {
		ms.m[id] = value
		if toWrite {
			ms.WriteLog()
		}
	}

}

func (ms *MapStorage) WriteLog() {
	jsonWrite, _ := json.MarshalIndent(ms.m, "", "  ")
	os.WriteFile(ms.f, jsonWrite, 0600)
}