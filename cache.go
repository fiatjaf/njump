package main

import (
	"encoding/json"
	"time"

	"github.com/dgraph-io/badger"
)

var cache = Cache{
	refreshTimers: make(chan struct{}),
	expiringKeys:  make(map[string]time.Time),
}

type Cache struct {
	*badger.DB

	refreshTimers chan struct{}
	expiringKeys  map[string]time.Time
}

func (c *Cache) initialize() func() {
	db, err := badger.Open(badger.DefaultOptions("/tmp/njump-cache"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open badger at /tmp/njump-cache")
	}
	c.DB = db

	// load expiringKeys
	err = c.DB.View(func(txn *badger.Txn) error {
		j, err := txn.Get([]byte("_expirations"))
		if err != nil {
			return err
		}

		expirations := make(map[string]int64)
		err = j.Value(func(val []byte) error {
			return json.Unmarshal(val, &expirations)
		})
		if err != nil {
			return err
		}

		for key, iwhen := range expirations {
			c.expiringKeys[key] = time.Unix(iwhen, 0)
		}

		return nil
	})
	if err != nil && err != badger.ErrKeyNotFound {
		panic(err)
	}

	go func() {
		// key expiration routine
		endOfTime := time.Unix(9999999999, 0)

		for {
			nextTimer := endOfTime

			for _, when := range c.expiringKeys {
				if when.Before(nextTimer) {
					nextTimer = when
				}
			}

			select {
			case <-time.After(nextTimer.Sub(time.Now())):
				// expire all keys that should have expired already
				now := time.Now()
				err := c.DB.Update(func(txn *badger.Txn) error {
					for key, when := range c.expiringKeys {
						if when.Before(now) {
							if err := txn.Delete([]byte(key)); err != nil {
								return err
							}
							delete(c.expiringKeys, key)
						}
					}
					return nil
				})
				if err != nil {
					log.Fatal().Err(err).Msg("")
				}
			case <-c.refreshTimers:
			}
		}
	}()

	// this is to be executed when the program ends
	return func() {
		// persist expiration times
		expirations := make(map[string]int64, len(c.expiringKeys))
		for key, when := range c.expiringKeys {
			expirations[key] = when.Unix()
		}
		j, _ := json.Marshal(expirations)
		err := c.DB.Update(func(txn *badger.Txn) error {
			return txn.Set([]byte("_expirations"), j)
		})
		if err != nil {
			panic(err)
		}
		db.Close()
	}
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

func (c *Cache) Set(key string, value []byte) {
	err := c.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func (c *Cache) SetWithTTL(key string, value []byte, ttl time.Duration) {
	err := c.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	c.expiringKeys[key] = time.Now().Add(ttl)
	c.refreshTimers <- struct{}{}
}
