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

// Worker implements a broadcasting mechanism for blocks.
// It creates a block on every new round and propagates it across the network, blocking until enough signatures will be
// collected(quorum is finalized)
type Worker struct {
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

func NewWorker(
	ctx context.Context,
	broadcaster rebro.Broadcaster,
	pool bapl.BatchPool,
	signerID crypto.PubKey,
	includers *stake.Includers,
) *Worker {
	ctx, cancel := context.WithCancel(ctx)
	return &Worker{
		ctx:         ctx,
		cancel:      cancel,
		broadcaster: broadcaster,
		batchPool:   pool,
		signerID:    signerID,
		includers:   includers,
	}
}

func (p *Worker) Start(_ context.Context) error {
	if p.log == nil {
		p.log = slog.Default()
	}
	go p.run()
	return nil
}

func (p *Worker) Stop(_ context.Context) error {
	if p.cancel == nil {
		return errors.New("already stooped")
	}

	p.cancel()
	p.cancel = nil
	return nil
}

// run is indefinitely producing new blocks and propagates them across the network
func (p *Worker) run() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		err := p.newRound()
		if err != nil {
			p.log.Error(err.Error())
			// temporary and hacky solution.
			// TODO: remove this in favour of better approach
			time.Sleep(time.Second * 3)
		}
	}
}

// newRound creates a new block and propagate it across the network.
// block creation consists of multiple stages:
// 1) collects all batches that has been produced by the signer;
// 2) creates block from batches and blocks from previous round;
// 3) propagates block and wait until quorum will be reached;
// 4) cleanups batches and stores block hashes from the current round;
func (p *Worker) newRound() error {
	batches, err := p.batchPool.ListBySigner(p.ctx, p.signerID.Bytes())
	if err != nil {
		return fmt.Errorf("can't get batches for the new round:%w", err)
	}

	confirmedBlockHashes := make([][]byte, len(p.certificates[p.round-1]))

	for i := range confirmedBlockHashes {
		confirmedBlockHashes[i] = p.certificates[p.round-1][i].Message().ID.Hash()
	}

	block := dag.NewBlock(p.round, p.signerID.Bytes(), batches, confirmedBlockHashes)
	quorum := stake.NewQuorum(p.includers)
	msg := rebro.Message{ID: block.ID(), Data: block.Hash()}

	err = p.broadcaster.Broadcast(p.ctx, msg, quorum)
	if err != nil {
		return err
	}

	for _, batch := range batches {
		err := p.batchPool.Delete(p.ctx, batch.Hash())
		if err != nil {
			p.log.Warn("can't delete a batch", err)
		}
	}

	p.certificates[p.round] = quorum.List()
	p.round++
	return nil
}
