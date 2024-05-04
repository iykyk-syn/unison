package bapl

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"capnproto.org/go/capnp/v3"
	"github.com/iykyk-syn/unison/bapl/batchmsg"
	"github.com/iykyk-syn/unison/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var defaultProtocolID = protocol.ID("/multicastpool/v0.0.1")

type FetchIncludersFn func() []peer.ID

type MulticastPool struct {
	pool      BatchPool
	host      host.Host
	includers FetchIncludersFn
	verifier  BatchVerifier
	signer    crypto.Signer

	protocolID protocol.ID

	log *slog.Logger
}

func NewMulticastPool(pool BatchPool, host host.Host, includers FetchIncludersFn, signer crypto.Signer, verifier BatchVerifier) *MulticastPool {
	return &MulticastPool{
		pool:       pool,
		host:       host,
		includers:  includers,
		verifier:   verifier,
		signer:     signer,
		protocolID: defaultProtocolID,
		log:        slog.With("module", "mcast-pool"),
	}
}

func (p *MulticastPool) Start() {
	p.host.SetStreamHandler(p.protocolID, func(stream network.Stream) {
		if err := p.rcvBatch(stream); err != nil {
			p.log.Error("receiving Batch", "err", err)
		}
	})
	p.log.Debug("started")
}

func (p *MulticastPool) Stop() {
	p.host.RemoveStreamHandler(p.protocolID)
}

func (p *MulticastPool) Push(ctx context.Context, batch *Batch) error {
	if batch.Signature.Body == nil {
		sig, err := p.signer.Sign(batch.Data)
		if err != nil {
			return err
		}

		batch.Signature = sig
	}

	if err := p.pool.Push(ctx, batch); err != nil {
		return err
	}

	if err := p.multicastBatch(ctx, batch); err != nil {
		return err
	}

	return nil
}

func (p *MulticastPool) Pull(ctx context.Context, hash []byte) (*Batch, error) {
	return p.pool.Pull(ctx, hash)
}

func (p *MulticastPool) ListBySigner(ctx context.Context, bytes []byte) ([]*Batch, error) {
	return p.pool.ListBySigner(ctx, bytes)
}

func (p *MulticastPool) Delete(ctx context.Context, hash []byte) error {
	return p.pool.Delete(ctx, hash)
}

func (p *MulticastPool) Size(ctx context.Context) (int, error) {
	return p.pool.Size(ctx)
}

func (p *MulticastPool) multicastBatch(ctx context.Context, batch *Batch) error {
	recipients := p.includers()
	if len(recipients) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(len(recipients))
	for _, r := range recipients {
		go func(r peer.ID) {
			defer wg.Done()
			if err := p.sendBatch(ctx, batch, r); err != nil {
				p.log.ErrorContext(ctx, "sending Batch", "err", err)
			}
		}(r)
	}

	// TODO: Wg does not respect context, rework with channels
	wg.Wait()
	return nil
}

func (p *MulticastPool) sendBatch(ctx context.Context, batch *Batch, to peer.ID) error {
	stream, err := p.host.NewStream(ctx, to, p.protocolID)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// set stream deadline from the context deadline.
	// if it is empty, then we assume that it will
	// hang until the server will close the stream by the timeout.
	if dl, ok := ctx.Deadline(); ok {
		if err = stream.SetDeadline(dl); err != nil {
			p.log.WarnContext(ctx, "error setting deadline", "err", err)
		}
	}

	msgMsg, msgSegment, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return err
	}

	msg, err := batchmsg.NewRootBatch(msgSegment)
	if err != nil {
		return err
	}

	err = msg.SetData(batch.Data)
	if err != nil {
		return err
	}

	err = msg.Signature().SetSignature(batch.Signature.Body)
	if err != nil {
		return err
	}

	err = msg.Signature().SetSigner(batch.Signature.Signer)
	if err != nil {
		return err
	}

	bytes, err := msgMsg.Marshal()
	if err != nil {
		return err
	}

	if _, err = stream.Write(bytes); err != nil {
		return fmt.Errorf("writing Batch to stream: %w", err)
	}
	if err = stream.CloseWrite(); err != nil {
		return err
	}
	// await ack from the other side
	if _, err = stream.Read([]byte{0}); err != nil && err != io.EOF {
		return fmt.Errorf("awaiting acknowledgement: %w", err)
	}

	return nil
}

func (p *MulticastPool) rcvBatch(s network.Stream) error {
	// TODO: SetDeadline

	batchData, err := io.ReadAll(s)
	if err != nil {
		return fmt.Errorf("reading Batch: %w", err)
	}

	msgMsg, err := capnp.Unmarshal(batchData)
	if err != nil {
		return err
	}

	msg, err := batchmsg.ReadRootBatch(msgMsg)
	if err != nil {
		return err
	}

	// ack other side that we are done by closing the stream
	if err = s.Close(); err != nil {
		return fmt.Errorf("closing Stream: %w", err)
	}

	batch := &Batch{Signature: crypto.Signature{}}
	batch.Data, err = msg.Data()
	if err != nil {
		return err
	}
	batch.Signature.Body, err = msg.Signature().Signature()
	if err != nil {
		return err
	}
	batch.Signature.Signer, err = msg.Signature().Signer()
	if err != nil {
		return err
	}

	// TODO: Must also verify the Signer is the set of active of includer
	err = p.signer.Verify(batch.Data, batch.Signature)
	if err != nil {
		return err
	}

	ok, err := p.verifier.Verify(context.TODO(), batch)
	if err != nil {
		return fmt.Errorf("verifying Batch: %w", err)
	}
	if !ok {
		return fmt.Errorf("batch verification failed")
	}

	if err = p.pool.Push(context.TODO(), batch); err != nil {
		return fmt.Errorf("pushing Batch: %w", err)
	}

	return s.Close()
}
