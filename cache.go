//go:build !nocache

package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dgraph-io/badger"
)

var cache = Cache{}

type Cache struct {
	*badger.DB
}

func (c *Cache) initialize() func() {
	db, err := badger.Open(badger.DefaultOptions("/tmp/njump-cache"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open badger at /tmp/njump-cache")
	}
	c.DB = db

	go func() {
		ticker := time.NewTicker(2 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
		again:
			err := db.RunValueLogGC(0.8)
			if err == nil {
				goto again
			}
		}
	}()

	return func() { db.Close() }
}

func (c *Cache) Get(key string) ([]byte, bool) {
	var val []byte
	err := c.DB.View(func(txn *badger.Txn) error {
		b, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		val, err = b.ValueCopy(nil)
		return err
	})

	if err == badger.ErrKeyNotFound {
		return nil, false
	}
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	return val, true
}

func (c *Cache) GetPaginatedkeys(prefix string, page int, size int) []string {
	keys := []string{}
	err := c.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		start := (page-1)*size + 1
		index := 1
		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			if index < start {
				index++
				continue
			}
			if index > start+size-1 {
				break
			}
			item := it.Item()
			k := item.Key()
			keys = append(keys, strings.TrimPrefix(string(k), prefix+":"))
			index++
		}
		return nil
	})

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	return keys
}

func (c *Cache) GetJSON(key string, recv any) bool {
	b, ok := c.Get(key)
	if !ok {
		return ok
	}
	json.Unmarshal(b, recv)
	return true
}

func (c *Cache) Set(key string, value []byte) {
	err := c.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func (c *Cache) SetJSON(key string, value any) {
	j, _ := json.Marshal(value)
	c.Set(key, j)
}

func (c *Cache) SetWithTTL(key string, value []byte, ttl time.Duration) {
	err := c.DB.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(
			badger.NewEntry([]byte(key), value).WithTTL(ttl),
		)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func (c *Cache) SetJSONWithTTL(key string, value any, ttl time.Duration) {
	j, _ := json.Marshal(value)
	c.SetWithTTL(key, j, ttl)
}
