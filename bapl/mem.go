package bapl

import (
	"bytes"
	"context"
	"sync"
)

// TODO needs:
//  * GC
//  *

type MemPool struct {
	batchesMu   sync.Mutex
	batchesCond sync.Cond
	batches     map[string]*Batch
	batchesSubs map[string]map[chan *Batch]struct{}
}

func (p *MemPool) Size(context.Context) (int, error) {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()
	return len(p.batches), nil
}

func NewMemPool() *MemPool {
	pool := &MemPool{
		batches:     make(map[string]*Batch),
		batchesSubs: make(map[string]map[chan *Batch]struct{}),
	}
	pool.batchesCond.L = &pool.batchesMu
	return pool
}

func (p *MemPool) Push(_ context.Context, batch *Batch) error {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()
	defer p.batchesCond.Broadcast()

	key := string(batch.Hash())
	p.batches[key] = batch

	subs, ok := p.batchesSubs[key]
	if ok {
		for sub := range subs {
			sub <- batch // subs are always buffered, so this won't block
		}
		delete(p.batchesSubs, key)
	}

	p.batches[key] = batch
	return nil
}

func (p *MemPool) Pull(ctx context.Context, hash []byte) (*Batch, error) {
	p.batchesMu.Lock()
	key := string(hash)
	r, ok := p.batches[key]
	if ok {
		p.batchesMu.Unlock()
		return r, nil
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
	case resp := <-sub:
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

func (p *MemPool) ListBySigner(_ context.Context, key []byte) ([]*Batch, error) {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()

	// TODO: Rework data structure to be O(1)
	for {
		var batches []*Batch
		for _, b := range p.batches {
			if bytes.Equal(b.Signature.Signer, key) {
				batches = append(batches, b)
			}
		}

		if len(batches) == 0 {
			p.batchesCond.Wait()
			continue
		}

		return batches, nil
	}
}

func (p *MemPool) Delete(_ context.Context, hash []byte) error {
	p.batchesMu.Lock()
	defer p.batchesMu.Unlock()

	key := string(hash)
	delete(p.batches, key)
	delete(p.batchesSubs, key)
	return nil
}
