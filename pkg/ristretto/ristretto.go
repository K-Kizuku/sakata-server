package ristretto

import "github.com/dgraph-io/ristretto"

func New() *ristretto.Cache {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,
		MaxCost:     1 << 20,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	return cache
}
