package round

import (
	"context"
	"errors"
	"sync"

	"github.com/iykyk-syn/unison/rebro"
)

// ErrElapsedRound is thrown when a requested height was already provided to [Manager].
var ErrElapsedRound = errors.New("elapsed round")

// Manager registers and manages lifecycles for every new [Round].
// It also provides a simple subscription mechanism in [Manager.GetRound] operations which are fulfilled
// with [Manager.NewRound].
type Manager struct {
	roundsMu    sync.Mutex
	rounds      map[uint64]*Round
	roundSubs   map[uint64]map[chan *Round]struct{}
	latestRound uint64
}

// NewManager instantiates a new [Manager].
func NewManager() *Manager {
	return &Manager{
		rounds:    make(map[uint64]*Round),
		roundSubs: make(map[uint64]map[chan *Round]struct{}),
	}
}

// NewRound instantiates a new [Round].
// It adds the [Round] to the [Manager], notifying all the [Manager.GetRound] waiters.
// and atomically stops previous latest round if running.
func (rm *Manager) NewRound(ctx context.Context, roundNum uint64, qcomm rebro.QuorumCertificate) (*Round, error) {
	rm.roundsMu.Lock()
	defer rm.roundsMu.Unlock()

	if rm.latestRound >= roundNum {
		return nil, ErrElapsedRound
	}
	// stop latest round if exist
	if r, ok := rm.rounds[rm.latestRound]; ok {
		if err := rm.stopRound(ctx, r); err != nil {
			return nil, err
		}
	}
	// create the new round and notify all the subscribers
	r := NewRound(roundNum, qcomm)
	subs, ok := rm.roundSubs[roundNum]
	if ok {
		for sub := range subs {
			sub <- r // subs are always buffered, so this won't block
		}
		delete(rm.roundSubs, roundNum)
	}

	rm.rounds[roundNum] = r
	rm.latestRound = roundNum
	return r, nil
}

// GetRound gets [Round] from local map by the number or subscribes for the [Round] to come, if not found.
func (rm *Manager) GetRound(ctx context.Context, roundNum uint64) (*Round, error) {
	rm.roundsMu.Lock()
	if rm.latestRound > roundNum {
		rm.roundsMu.Unlock()
		return nil, ErrElapsedRound
	}

	r, ok := rm.rounds[roundNum]
	if ok {
		rm.roundsMu.Unlock()
		return r, nil
	}

	subs, ok := rm.roundSubs[roundNum]
	if !ok {
		subs = make(map[chan *Round]struct{})
		rm.roundSubs[roundNum] = subs
	}

	sub := make(chan *Round, 1)
	subs[sub] = struct{}{}
	rm.roundsMu.Unlock()

	select {
	case resp, ok := <-sub:
		if !ok {
			return nil, ErrElapsedRound
		}
		return resp, nil
	case <-ctx.Done():
		// no need to keep the request, if the caller has canceled
		rm.roundsMu.Lock()
		delete(subs, sub)
		if len(subs) == 0 {
			delete(rm.roundSubs, roundNum)
		}
		rm.roundsMu.Unlock()
		return nil, ctx.Err()
	}
}

// Stop performs [Round.Finalize] and [Round.Stop] on all the registered instances of [Round] and
// then terminates. This ensures we retain in-progress [Round] state.
func (rm *Manager) Stop(ctx context.Context) error {
	// lock manager fully and prevent any other actions Round while we stop
	rm.roundsMu.Lock()
	defer rm.roundsMu.Unlock()

	for _, r := range rm.rounds {
		err := r.Finalize(ctx)
		if err != nil {
			return err
		}

		err = rm.stopRound(ctx, r)
		if err != nil {
			return err
		}
	}

	return nil
}

// stopRound stops [Round] and deletes it from the [Manager] together with the active subscriptions
// for it. It does not wait for the [Round] finalization and that's a caller's concern.
func (rm *Manager) stopRound(ctx context.Context, r *Round) error {
	err := r.Stop(ctx)
	if err != nil {
		return err
	}

	delete(rm.rounds, r.RoundNumber())
	for sub := range rm.roundSubs[r.RoundNumber()] {
		close(sub)
	}
	delete(rm.roundSubs, r.RoundNumber())
	return nil
}
