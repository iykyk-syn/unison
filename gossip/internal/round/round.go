package round

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1ykyk/rebro"
)

const (
	stateOperationsChannelSize      = 32
	subscriptionCancellationTimeout = time.Second
)

// errClosedRound singles that Round is accessed after being closed
var errClosedRound = errors.New("closed round access")

// Round maintains state of broadcasting rounds and local pubsub system for commitments.
// It guards QuorumCommitment from concurrent access ensuring thread-safety.
// Round is not concerned of validity of any given input and solely acting as a state machine of
// the broadcasting round.
type Round struct {
	// the actual state of the Round
	quorum rebro.QuorumCommitment

	// channel for operation submission to be executed
	stateOpCh chan *stateOp
	// maintains subscriptions for commitments by their ids
	getOpSubs map[string]map[*stateOp]struct{}

	// signalling for graceful shutdown
	closeCh, closedCh chan struct{}
}

// NewRound instantiates Round state machine of QuorumCommitment.
func NewRound(quorum rebro.QuorumCommitment) *Round {
	r := &Round{
		quorum:    quorum,
		stateOpCh: make(chan *stateOp, stateOperationsChannelSize),
		getOpSubs: make(map[string]map[*stateOp]struct{}),
		closeCh:   make(chan struct{}),
		closedCh:  make(chan struct{}),
	}
	go r.stateLoop()
	return r
}

// Stop gracefully stops the Round allowing early termination through context.
// It ensures all the in-progress state operations are completed before termination.
func (r *Round) Stop(ctx context.Context) error {
	select {
	case <-r.closeCh:
		return errClosedRound
	default:
	}

	close(r.closeCh)
	select {
	case <-r.closedCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AddCommitment add the given message to the Round forming a new commitment.
func (r *Round) AddCommitment(ctx context.Context, msg rebro.Message) error {
	op := newStateOp(addOp)
	defer op.Free()

	op.msg = &msg
	return r.execOp(ctx, op)
}

// stateAdd adds commitment to quorum additionally notifying all the subscribers for this commitment.
func (r *Round) stateAdd(op *stateOp) {
	err := r.quorum.Add(*op.msg)
	if err != nil {
		op.SetError(err)
		return
	}
	// we added, now lets see if there were any subscribers
	key := op.msg.ID.String()
	if len(r.getOpSubs[key]) == 0 {
		op.SetError(nil)
		return
	}
	// if so, get the commitment
	comm, ok := r.quorum.Get(op.msg.ID)
	if !ok {
		panic("commitment not found on Get after successful Put")
	}
	// and notify them
	for op := range r.getOpSubs[key] {
		op.SetCommitment(comm)
	}
	// cleaning up the subscriptions
	delete(r.getOpSubs, key)
	// and finishing the main operation
	op.SetError(nil)
}

// GetCommitment gets commitment from the Round by the associated Message's ID.
func (r *Round) GetCommitment(ctx context.Context, id rebro.MessageID) (rebro.Commitment, error) {
	op := newStateOp(getOp)
	defer op.Free()

	op.id = id
	err := r.execOp(ctx, op)
	if err == nil || ctx.Err() == nil {
		return op.comm, err
	}
	// if the context got cancelled for any reason,
	// cancel the get subscription by executing the op over
	//
	// set a timeout, as the main context got cancelled already,
	// and we want to prevent the caller from waiting indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), subscriptionCancellationTimeout)
	err = errors.Join(err, r.execOp(ctx, op))
	cancel()
	return nil, err
}

// stateGet gets commitment from quorum or subscribes to the commitment until it comes
func (r *Round) stateGet(op *stateOp) {
	comm, ok := r.quorum.Get(op.id)
	if ok {
		op.SetCommitment(comm)
		return
	}

	key := op.id.String()
	inner, ok := r.getOpSubs[key]
	if !ok {
		inner = make(map[*stateOp]struct{})
		r.getOpSubs[key] = inner
	}
	// check if we had that operation before
	if _, ok = inner[op]; !ok {
		// if not, keep the op as subscription
		inner[op] = struct{}{}
		return
	}
	// if so, that's a subscription cancellation so delete the entry
	delete(inner, op)
	// cleanup the main map if it is empty
	if len(inner) == 0 {
		delete(r.getOpSubs, key)
	}
	op.SetError(nil)
	return
}

// DeleteCommitment deletes commitment from the Round by the associated Message's ID.
func (r *Round) DeleteCommitment(ctx context.Context, id rebro.MessageID) error {
	op := newStateOp(deleteOp)
	defer op.Free()

	op.id = id
	return r.execOp(ctx, op)
}

func (r *Round) stateDelete(op *stateOp) {
	ok := r.quorum.Delete(op.id) // TODO: Maybe error instead?
	if !ok {
		op.SetError(fmt.Errorf("coudn't delete Commitment"))
		return
	}
	op.SetError(nil)
}

// AddSignature appends a Signature to one of the Round's Commitments.
func (r *Round) AddSignature(ctx context.Context, id rebro.MessageID, sig rebro.Signature) error {
	op := newStateOp(addSignOp)
	defer op.Free()

	op.id = id
	op.sig = &sig
	return r.execOp(ctx, op)
}

func (r *Round) stateAddSign(op *stateOp) {
	comm, ok := r.quorum.Get(op.id)
	if !ok {
		op.SetError(fmt.Errorf("coudn't find Commitment"))
		return
	}

	_, err := comm.AddSignature(*op.sig)
	if !ok {
		op.SetError(err)
		return
	}
	op.SetError(nil)
}

// execOp submits operation for execution by stateLoop and awaits for its completion
// It permits submission until closedCh is closed or context is cancelled, even after closing is
// triggered. This allows some "last-minute" operations to "squeeze in" before Round fully finishes.
func (r *Round) execOp(ctx context.Context, op *stateOp) error {
	select {
	case r.stateOpCh <- op:
	case <-r.closedCh:
		return errClosedRound
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case <-op.doneCh:
		return op.err
	case <-r.closedCh:
		return errClosedRound
	case <-ctx.Done():
		return ctx.Err()
	}
}

// execOpAsync is equivalent to execOp but async, meaning it does not wait until op has finished.
// used for testing only
func (r *Round) execOpAsync(ctx context.Context, op *stateOp) error {
	select {
	case r.stateOpCh <- op:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// stateLoop is an event loop performing state operations on the round
// and ensures access to QuorumCommitment is single-threaded
func (r *Round) stateLoop() {
	doOp := func(op *stateOp) {
		switch op.kind {
		case addOp:
			r.stateAdd(op)
		case getOp:
			r.stateGet(op)
		case deleteOp:
			r.stateDelete(op)
		case addSignOp:
			r.stateAddSign(op)
		default:
			panic("unknown operation type")
		}
	}

	defer func() {
		// this mechanism ensures we drain the channel
		// and execute all the pending ops before we fully close
		for {
			select {
			case op := <-r.stateOpCh:
				doOp(op)
			default:
				close(r.closedCh)
				return
			}
		}
	}()

	for {
		select {
		case op := <-r.stateOpCh:
			doOp(op)
		case <-r.closeCh:
			return
		}
	}
}
