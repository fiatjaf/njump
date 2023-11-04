//go:build nocache

package main

import (
	"time"
)

var cache = Cache{}

type Cache struct{}

func (c *Cache) initialize() func()                                          { return func() {} }
func (c *Cache) Get(key string) ([]byte, bool)                               { return nil, false }
func (c *Cache) GetJSON(key string, recv any) bool                           { return false }
func (c *Cache) Set(key string, value []byte)                                {}
func (c *Cache) SetJSON(key string, value any)                               {}
func (c *Cache) SetWithTTL(key string, value []byte, ttl time.Duration)      {}
func (c *Cache) SetJSONWithTTL(key string, value any, ttl time.Duration)     {}
func (c *Cache) GetPaginatedKeys(prefix string, page int, size int) []string { return []string{} }
