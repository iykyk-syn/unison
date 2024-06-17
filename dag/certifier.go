package dag

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/iykyk-syn/unison/bapl"
	"github.com/iykyk-syn/unison/dag/block"
	"github.com/iykyk-syn/unison/rebro"
)

type certifier struct {
	pool bapl.BatchPool
	log  *slog.Logger
}

func NewCertifier(pool bapl.BatchPool) rebro.Certifier {
	return &certifier{pool: pool, log: slog.With("module", "certifiers")}
}

func (c *certifier) Certify(ctx context.Context, msg rebro.Message) error {
	if msg.Data == nil {
		return errors.New("block data is empty")
	}
	err := msg.ID.Validate()
	if err != nil {
		return fmt.Errorf("validating blockID:%w", err)
	}

	blk := &block.Block{}
	err = blk.UnmarshalBinary(msg.Data)
	if err != nil {
		return fmt.Errorf("unmarshalling block %w", err)
	}

	err = blk.Validate()
	if err != nil {
		return fmt.Errorf("validating block %w", err)
	}

	for _, hash := range blk.Batches() {
		_, err = c.pool.Pull(ctx, hash)
		if err != nil && !errors.Is(err, bapl.ErrBatchDeleted) { // TODO: This is a temporary workaround
			return fmt.Errorf("getting bacth hash %w", err)
		}
	}

	c.log.Debug("certified", "block_hash", blk)
	return nil
}
