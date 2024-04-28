package dag

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/iykyk-syn/unison/bapl"
	"github.com/iykyk-syn/unison/crypto"
	dag "github.com/iykyk-syn/unison/dag/block"
	"github.com/iykyk-syn/unison/rebro"
	"github.com/iykyk-syn/unison/stake"
)

type IncludersFn func(round uint64) (*stake.Includers, error)

// Dagger implements a broadcasting mechanism for blocks.
// It creates a block on every new round and propagates it across the network, blocking until enough signatures will be
// collected(quorum is finalized)
type Dagger struct {
	broadcaster rebro.Broadcaster
	batchPool   bapl.BatchPool
	includers   IncludersFn

	signerID     crypto.PubKey
	blockTimeout time.Duration

	round      uint64
	lastQuorum rebro.QuorumCertificate

	log    *slog.Logger
	cancel context.CancelFunc
}

func NewDagger(
	broadcaster rebro.Broadcaster,
	pool bapl.BatchPool,
	includers IncludersFn,
	signerID crypto.PubKey,
	blockTimeout time.Duration,
) *Dagger {
	return &Dagger{
		broadcaster:  broadcaster,
		batchPool:    pool,
		includers:    includers,
		signerID:     signerID,
		blockTimeout: blockTimeout,
		round:        1, // must start from 1
		log:          slog.With("module", "dagger"),
	}
}

func (d *Dagger) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	go d.run(ctx)
	d.log.Debug("started")
	return
}

func (d *Dagger) Stop() {
	d.cancel()
}

// run is indefinitely producing new blocks and propagates them across the network
func (d *Dagger) run(ctx context.Context) {
	for ctx.Err() == nil {
		if d.blockTimeout != 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(d.blockTimeout):
			}
		}

		err := d.startRound(ctx)
		if err != nil {
			d.log.ErrorContext(ctx, "executing round", "reason", err)
			// temporary and hacky solution.
			// TODO: remove this in favour of better approach
			time.Sleep(time.Second * 3)
		}
	}
}

// startRound assembles a new block and broadcasts it across the network.
//
// assembling stages:
// * collect block hashes from last round as parent hashes
// * cleanup batches commited in blocks from last round
// * prepare the new uncommited batches
// * create a block from the batches and the parents hashes;
// * propagate the block and wait until quorum is reached;
func (d *Dagger) startRound(ctx context.Context) error {
	certs := d.lastCertificates()
	parents := make([][]byte, len(certs))
	for i, cert := range certs {
		parents[i] = certs[i].Message().ID.Hash()

		var block dag.Block
		err := block.UnmarshalBinary(cert.Message().Data)
		if err != nil {
			panic(err)
		}

		for _, batchHash := range block.Batches() {
			err := d.batchPool.Delete(ctx, batchHash)
			if err != nil {
				d.log.WarnContext(ctx, "can't delete a batch", "err", err)
			}
		}
	}

	newBatches, err := d.batchPool.ListBySigner(ctx, d.signerID.Bytes())
	if err != nil {
		return fmt.Errorf("can't get batches for the new round:%w", err)
	}

	// TODO: certificate signatures should be the part of the block.
	block := dag.NewBlock(d.round, d.signerID.Bytes(), newBatches, parents)
	block.Hash() // TODO: Compute in constructor
	data, err := block.MarshalBinary()
	if err != nil {
		return err
	}

	includers, err := d.includers(d.round)
	if err != nil {
		return err
	}

	msg := rebro.Message{ID: block.ID(), Data: data}
	quorum := stake.NewQuorum(includers)
	err = d.broadcaster.Broadcast(ctx, msg, quorum)
	if err != nil {
		return err
	}
	d.log.InfoContext(ctx, "finished round", "round", d.round, "batches", len(newBatches), "parents", len(parents))

	d.round++
	d.lastQuorum = quorum
	return nil
}

func (d *Dagger) lastCertificates() []rebro.Certificate {
	if d.lastQuorum == nil {
		return nil
	}

	return d.lastQuorum.List()
}
