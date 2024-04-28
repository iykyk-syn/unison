package main

import (
	"context"
	"crypto/rand"
	"log/slog"
	"time"

	"github.com/iykyk-syn/unison/bapl"
)

func RandomBatches(ctx context.Context, pool bapl.BatchPool, batchSize int, batchTime time.Duration) {
	ticker := time.NewTicker(batchTime)
	defer ticker.Stop()

	log := slog.With("module", "randomizer")
	for {
		select {
		case <-ticker.C:
			go func() {
				batchData := make([]byte, batchSize)
				rand.Read(batchData)
				batch := &bapl.Batch{Data: batchData}
				err := pool.Push(ctx, batch)
				if err != nil {
					log.ErrorContext(ctx, "error pushing batch", "err", err)
				}
				log.DebugContext(ctx, "pushed batch")
			}()

		case <-ctx.Done():
			return
		}
	}
}
