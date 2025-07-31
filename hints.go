package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"fiatjaf.com/nostr/sdk/hints/memoryh"
)

// save these things to a file so we can reload them later
func outboxHintsFileLoaderSaver(ctx context.Context) {
	if file, err := os.Open(s.HintsMemoryDumpPath); err == nil {
		hdb := memoryh.NewHintDB()
		if err := json.NewDecoder(file).Decode(&hdb); err == nil {
			sys.Hints = hdb
		}
		file.Close()
	}

	const tmp = "/tmp/njump-outbox-hints-tmp.json"

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute * 5):
		}

		hdb := sys.Hints.(*memoryh.HintDB)
		file, err := os.Create(tmp)
		if err != nil {
			log.Error().Err(err).Str("path", tmp).Msg("failed to create outbox hints file")
			time.Sleep(time.Hour)
			continue
		}
		file.Close()
		json.NewEncoder(file).Encode(hdb)
		if err := os.Rename(tmp, s.HintsMemoryDumpPath); err != nil {
			log.Error().Err(err).Str("from", tmp).Str("to", s.HintsMemoryDumpPath).Msg("failed to move outbox hints file")
			time.Sleep(time.Hour)
			continue
		}
	}
}
