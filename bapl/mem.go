package bapl

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"
)

var ErrBatchDeleted = errors.New("batch deleted")

type MemPool struct {
	batchesMu   sync.Mutex
	batchesCond sync.Cond
	batches     map[string]batchEntry
	batchesSubs map[string]map[chan *Batch]struct{}
	closeCh     chan struct{}
}

type batchEntry struct {
	*Batch
	time time.Time
}

func (p *MemPool) Size(context.Context) (int, error) {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()
	return len(p.batches), nil
}

func NewMemPool() *MemPool {
	pool := &MemPool{
		batches:     make(map[string]batchEntry),
		batchesSubs: make(map[string]map[chan *Batch]struct{}),
		closeCh:     make(chan struct{}),
	}
	pool.batchesCond.L = &pool.batchesMu
	go pool.gc()
	return pool
}

func (p *MemPool) Close() {
	close(p.closeCh)
}

func (p *MemPool) Push(_ context.Context, batch *Batch) error {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()
	defer p.batchesCond.Broadcast()

	key := string(batch.Hash())
	p.batches[key] = batchEntry{Batch: batch, time: time.Now()}

	subs, ok := p.batchesSubs[key]
	if ok {
		for sub := range subs {
			sub <- batch // subs are always buffered, so this won't block
		}
		delete(p.batchesSubs, key)
	}
	return nil
}

func (p *MemPool) Pull(ctx context.Context, hash []byte) (*Batch, error) {
	p.batchesMu.Lock()
	key := string(hash)
	r, ok := p.batches[key]
	if ok {
		p.batchesMu.Unlock()
		return r.Batch, nil
	}

	subs, ok := p.batchesSubs[key]
	if !ok {
		subs = make(map[chan *Batch]struct{})
		p.batchesSubs[key] = subs
	}

	sub := make(chan *Batch, 1)
	subs[sub] = struct{}{}
	p.batchesMu.Unlock()

	select {
	case resp, ok := <-sub:
		if !ok {
			return nil, ErrBatchDeleted
		}
		return resp, nil
	case <-ctx.Done():
		// no need to keep the request, if the caller has canceled
		p.batchesMu.Lock()
		delete(subs, sub)
		if len(subs) == 0 {
			delete(p.batchesSubs, key)
		}
		p.batchesMu.Unlock()
		return nil, ctx.Err()
	}
}

func (p *MemPool) ListBySigner(_ context.Context, signer []byte) ([]*Batch, error) {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()

	// TODO: Rework data structure to be O(1)
	for {
		var batches []*Batch
		for _, b := range p.batches {
			if bytes.Equal(b.Signature.Signer, signer) && !b.Included {
				batches = append(batches, b.Batch)
			}
		}

		if len(batches) > 0 {
			return batches, nil
		}

		p.batchesCond.Wait()
	}
}

func (p *MemPool) Delete(_ context.Context, hash []byte) error {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()

	key := string(hash)
	delete(p.batches, key)
	for sub := range p.batchesSubs[key] {
		close(sub)
	}
	delete(p.batchesSubs, key)
	return nil
}

var (
	gcTime     = time.Second * 10
	staleAfter = time.Minute
)

// gc periodically cleans up stale batches
func (p *MemPool) gc() {
	ticker := time.NewTicker(gcTime)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			p.batchesMu.Lock()
			for key, b := range p.batches {
				if b.time.Add(staleAfter).Before(now) {
					delete(p.batches, key)
					for sub := range p.batchesSubs[key] {
						close(sub)
					}
					delete(p.batchesSubs, key)
				}
			}
			p.batchesMu.Unlock()
		case <-p.closeCh:
			return
		}
	}
}
