package dag

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/iykyk-syn/unison/bapl"
	"github.com/iykyk-syn/unison/crypto"
	dag "github.com/iykyk-syn/unison/dag/block"
	"github.com/iykyk-syn/unison/rebro"
	"github.com/iykyk-syn/unison/stake"
)

// Dagger implements a broadcasting mechanism for blocks.
// It creates a block on every new round and propagates it across the network, blocking until enough signatures will be
// collected(quorum is finalized)
type Dagger struct {
	log *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc

	round uint64

	broadcaster rebro.Broadcaster

	batchPool bapl.BatchPool

	signerID crypto.PubKey

	certificates map[uint64][]rebro.Certificate

	includers *stake.Includers
}

func NewDagger(
	ctx context.Context,
	broadcaster rebro.Broadcaster,
	pool bapl.BatchPool,
	signerID crypto.PubKey,
	includers *stake.Includers,
) *Dagger {
	ctx, cancel := context.WithCancel(ctx)
	return &Dagger{
		ctx:         ctx,
		cancel:      cancel,
		broadcaster: broadcaster,
		batchPool:   pool,
		signerID:    signerID,
		includers:   includers,
	}
}

func (d *Dagger) Start(_ context.Context) error {
	if d.log == nil {
		d.log = slog.Default()
	}
	go d.run()
	return nil
}

func (d *Dagger) Stop(_ context.Context) error {
	if d.cancel == nil {
		return errors.New("already stooped")
	}

	d.cancel()
	d.cancel = nil
	return nil
}

// run is indefinitely producing new blocks and propagates them across the network
func (d *Dagger) run() {
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
		}

		err := d.startRound()
		if err != nil {
			d.log.Error(err.Error())
			// temporary and hacky solution.
			// TODO: remove this in favour of better approach
			time.Sleep(time.Second * 3)
		}
	}
}

// startRound creates a new block and propagate it across the network.
// block creation consists of multiple stages:
// 1) collects all batches that has been produced by the signer;
// 2) creates block from batches and blocks from previous round;
// 3) propagates block and wait until quorum will be reached;
// 4) cleanups batches and stores block hashes from the current round;
func (d *Dagger) startRound() error {
	batches, err := d.batchPool.ListBySigner(d.ctx, d.signerID.Bytes())
	if err != nil {
		return fmt.Errorf("can't get batches for the new round:%w", err)
	}

	confirmedBlockHashes := make([][]byte, len(d.certificates[d.round-1]))

	for i := range confirmedBlockHashes {
		confirmedBlockHashes[i] = d.certificates[d.round-1][i].Message().ID.Hash()
	}

	// TODO: certificate signatures should be the part of the block.
	block := dag.NewBlock(d.round, d.signerID.Bytes(), batches, confirmedBlockHashes)
	data, err := block.MarshalBinary()
	if err != nil {
		return err
	}
	quorum := stake.NewQuorum(d.includers)

	msg := rebro.Message{ID: block.ID(), Data: data}

	err = d.broadcaster.Broadcast(d.ctx, msg, quorum)
	if err != nil {
		return err
	}

	for _, batch := range batches {
		err := d.batchPool.Delete(d.ctx, batch.Hash())
		if err != nil {
			d.log.Warn("can't delete a batch", err)
		}
	}

	d.certificates[d.round] = quorum.List()
	d.round++
	return nil
}
