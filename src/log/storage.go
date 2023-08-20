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

	Get(key string) (interface{}, bool)

	// HasData returns true iff any Sets were made on this Storage.
	HasData() bool

	// GetLog returns the log of all Sets made on this Storage.
	GetLog() []map[string]interface{}
}

// MapStorage is a simple in-memory implementation of Storage for testing.
type MapStorage struct {
	mu sync.Mutex
	m  []map[string]interface{}
	f  string
}

func NewMapStorage() *MapStorage {
	m := make([]map[string]interface{}, 0)
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

func (ms *MapStorage) Get(key string) (interface{}, bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	v, found := ms.m[len(ms.m)-1][key]
	return v, found
}

func (ms *MapStorage) Set(value map[string]interface{}, toWrite bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.m = append(ms.m, value)

	if toWrite {
		ms.WriteLog()
	}

}

func (ms *MapStorage) WriteLog() {
	jsonWrite, _ := json.MarshalIndent(ms.m, "", "  ")
	os.WriteFile(ms.f, jsonWrite, 0600)
}

func (ms *MapStorage) HasData() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return len(ms.m) > 0
}

func (ms *MapStorage) GetLog() []map[string]interface{} {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.m
}