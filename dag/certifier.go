package dag

import (
	"context"
	"errors"
	"fmt"

	"github.com/iykyk-syn/unison/bapl"
	dag "github.com/iykyk-syn/unison/dag/block"
	"github.com/iykyk-syn/unison/rebro"
)

type certifier struct {
	pool bapl.BatchPool
}

func NewCertifier(pool bapl.BatchPool) rebro.Certifier {
	return &certifier{pool: pool}
}

func (v *certifier) Certify(ctx context.Context, msg rebro.Message) error {
	if msg.Data == nil {
		return errors.New("block data is empty")
	}
	err := msg.ID.Validate()
	if err != nil {
		return fmt.Errorf("validating blockID:%v", err)
	}

	block := &dag.Block{}
	err = block.UnmarshalBinary(msg.Data)
	if err != nil {
		return fmt.Errorf("unmarshalling block %v", err)
	}

	err = block.Validate()
	if err != nil {
		return fmt.Errorf("validating block %v", err)
	}

	for _, hash := range block.Batches() {
		_, err = v.pool.Pull(ctx, hash)
		if err != nil {
			return fmt.Errorf("getting bacth hash %v", err)
		}
	}
	return nil
}
