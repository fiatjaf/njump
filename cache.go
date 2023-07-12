package main

import (
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

func (c *Cache) initialize() func() error {
	db, err := badger.Open(badger.DefaultOptions("/tmp/njump-cache"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open badger at /tmp/njump-cache")
	}
	c.DB = db

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
					panic(err)
				}
			case <-c.refreshTimers:
			}
		}
	}()

	return db.Close
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
		panic(err)
	}

	return val, true
}

func (c *Cache) Set(key string, value []byte) {
	err := c.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	if err != nil {
		panic(err)
	}
}

func (c *Cache) SetWithTTL(key string, value []byte, ttl time.Duration) {
	err := c.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	if err != nil {
		panic(err)
	}
	c.expiringKeys[key] = time.Now().Add(ttl)
	c.refreshTimers <- struct{}{}
}
