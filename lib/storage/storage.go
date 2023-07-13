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
	Set(value map[string]string)

	Get(key string) (string, bool)

	// HasData returns true iff any Sets were made on this Storage.
	HasData() bool
}

// MapStorage is a simple in-memory implementation of Storage for testing.
type MapStorage struct {
	mu sync.Mutex
	m  []map[string]string
	f  string
}

func NewMapStorage() *MapStorage {
	m := make([]map[string]string, 0)
	ms := &MapStorage{
		m: m,
		f: "log.txt",
	}

	jsonRead, err := os.ReadFile(ms.f)
	if err != nil {
		os.OpenFile(ms.f, os.O_CREATE|os.O_WRONLY, 0600)
		os.WriteFile(ms.f, []byte("[]"), 0600)
	} else {
		json.Unmarshal(jsonRead, &ms.m)
	}

	return ms

}

func (ms *MapStorage) Get(key string) (string, bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	v, found := ms.m[len(ms.m)-1][key]
	return v, found
}

func (ms *MapStorage) Set(value map[string]string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.m = append(ms.m, value)

	fd, err := os.OpenFile(ms.f, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	if len(ms.m) > 1 {
		fd.Seek(-2, 2)
		fd.WriteString(",")
	} else {
		fd.Seek(-1, 2)
	}

	fd.Write([]byte("\n    "))
	b, _ := json.MarshalIndent(value, "    ", "    ")
	fd.Write(b)
	fd.Write([]byte("\n]"))

}

func (ms *MapStorage) HasData() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return len(ms.m) > 0
}