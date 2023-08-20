// Eli Bendersky [https://eli.thegreenplace.net]
// This code is in the public domain.
package storage

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

// Storage is an interface implemented by stable storage providers.
type Storage interface {
	Set(value map[string]interface{}, toWrite bool)

	Get(key string) (interface{}, bool)

	// HasData returns true iff any Sets were made on this Storage.
	HasData() bool

	// GetLog returns the log of all Sets made on this Storage.
	GetLog() map[string]map[string]interface{}
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

func (ms *MapStorage) Get(key string) (interface{}, bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	times := []time.Time{}
	for _, v := range ms.m {
		time, _ := time.Parse("2006-01-02 15:04:05.0000", v["Time"].(string))
		times = append(times, time)
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})
	v, found := ms.m[times[len(times)-1].String()][key]
	return v, found
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

func (ms *MapStorage) HasData() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return len(ms.m) > 0
}

func (ms *MapStorage) GetLog() map[string]map[string]interface{} {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.m
}