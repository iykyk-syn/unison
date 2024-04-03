package round

import (
	"context"
	"errors"
	"sync"

	"github.com/1ykyk/rebro"
)

// errElapsedRound is thrown when a requested height was already provided to [Manager].
var errElapsedRound = errors.New("elapsed round")

// Manager registers and manages lifecycles for every new [Round].
// It also provides a simple subscription mechanism in [Manager.GetRound] operations which are fulfilled
// with [Manager.StartRound].
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

		err = rm.StopRound(ctx, r.RoundNumber())
		if err != nil {
			return err
		}
	}

	return nil
}

// StartRound instantiates and starts a new [Round].
// It adds the [Round] to the [Manager], notifying all the [Manager.GetRound] waiters.
func (rm *Manager) StartRound(roundNum uint64, qcomm rebro.QuorumCommitment) (*Round, error) {
	rm.roundsMu.Lock()
	defer rm.roundsMu.Unlock()

	if rm.latestRound >= roundNum {
		return nil, errElapsedRound
	}
	rm.latestRound = roundNum

	r := NewRound(roundNum, qcomm)
	subs, ok := rm.roundSubs[roundNum]
	if ok {
		for sub := range subs {
			sub <- r // subs are always buffered, so this won't block
		}
		delete(rm.roundSubs, roundNum)
	}

	rm.rounds[roundNum] = r
	return r, nil
}

// StopRound stops [Round] and deletes it from the [Manager] together with the active subscriptions for it.
// It does not wait for the [Round] finalization and that's a caller's concern.
func (rm *Manager) StopRound(ctx context.Context, roundNum uint64) error {
	rm.roundsMu.Lock()
	if rm.latestRound >= roundNum {
		return errElapsedRound
	}
	r := rm.rounds[roundNum]
	rm.roundsMu.Unlock()

	err := r.Stop(ctx)
	if err != nil {
		return err
	}

	rm.roundsMu.Lock()
	delete(rm.rounds, roundNum)
	delete(rm.roundSubs, roundNum)
	rm.roundsMu.Unlock()
	return nil
}

// GetRound gets [Round] from local map by the number or subscribes for the [Round] to come, if not found.
func (rm *Manager) GetRound(ctx context.Context, roundNum uint64) (*Round, error) {
	rm.roundsMu.Lock()
	r, ok := rm.rounds[roundNum]
	if ok {
		rm.roundsMu.Unlock()
		return r, nil
	}
	// check for elapsed round only after the rounds map were checked.
	if rm.latestRound >= roundNum {
		rm.roundsMu.Unlock()
		return nil, errElapsedRound
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
	case resp := <-sub:
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
