package round

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

const (
	stateOperationsChannelSize      = 32
	subscriptionCancellationTimeout = time.Second
)

// errClosedRound singles that Round is accessed after being closed
var errClosedRound = errors.New("closed round access")

// Round maintains state of broadcasting rounds and local pubsub system for certificates.
// It guards [rebro.QuorumCertificate] from concurrent access ensuring thread-safety.
// Round is not concerned of validity of any given input and solely acting as a state machine of
// the broadcasting round.
type Round struct {
	roundNum uint64
	// the actual state of the Round
	quorum rebro.QuorumCertificate

	// channel for operation submission to be executed
	stateOpCh chan *stateOp
	// maintains subscriptions for certificates by their ids
	getOpSubs map[string]map[*stateOp]struct{}
	// finalCh gets closed when the quorum certificate has been finalized to notify listeners
	finalCh chan struct{}
	// signalling for graceful shutdown
	closeCh, closedCh chan struct{}
}

// NewRound instantiates a new [Round] state machine wrapping [rebro.QuorumCertificate].
// This passes full ownership of the [rebro.QuorumCertificate] fully to [Round],
// thus it must not be used for writes until [Round] has been stopped.
func NewRound(roundNum uint64, quorum rebro.QuorumCertificate) *Round {
	r := &Round{
		roundNum:  roundNum,
		quorum:    quorum,
		stateOpCh: make(chan *stateOp, stateOperationsChannelSize),
		getOpSubs: make(map[string]map[*stateOp]struct{}),
		finalCh:   make(chan struct{}),
		closeCh:   make(chan struct{}),
		closedCh:  make(chan struct{}),
	}
	go r.stateLoop()
	return r
}

// Stop gracefully stops the [Round] allowing early termination through context.
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

// Finalize awaits finalization of the [Round]'s [rebro.QuorumCertificate].
func (r *Round) Finalize(ctx context.Context) error {
	select {
	case <-r.finalCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RoundNumber provides number of the [Round].
func (r *Round) RoundNumber() uint64 {
	return r.roundNum
}

// AddCertificate add the given message to the Round forming a new [rebro.Certificate] on
// [rebro.QuorumCertificate].
func (r *Round) AddCertificate(ctx context.Context, msg rebro.Message) error {
	op := newStateOp(addOp)
	op.msg = &msg

	return r.execOp(ctx, op)
}

// stateAdd adds certificate to quorum additionally notifying all the subscribers for this certificate.
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
	// if so, get the certificate
	comm, ok := r.quorum.Get(op.msg.ID)
	if !ok {
		panic("certificate not found on Get after successful Put")
	}
	// and notify them
	for op := range r.getOpSubs[key] {
		op.SetCertificate(comm)
	}
	// cleaning up the subscriptions
	delete(r.getOpSubs, key)
	// and finishing the main operation
	op.SetError(nil)
}

// GetCertificate gets certificate from the [Round] by the associated [rebro.MessageID].
func (r *Round) GetCertificate(ctx context.Context, id rebro.MessageID) (rebro.Certificate, error) {
	op := newStateOp(getOp)
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

// stateGet gets certificate from quorum or subscribes to the certificate until it comes
func (r *Round) stateGet(op *stateOp) {
	comm, ok := r.quorum.Get(op.id)
	if ok {
		op.SetCertificate(comm)
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

// DeleteCertificate deletes certificate from the [Round] by the associated [rebro.MessageID].
func (r *Round) DeleteCertificate(ctx context.Context, id rebro.MessageID) error {
	op := newStateOp(deleteOp)
	op.id = id

	return r.execOp(ctx, op)
}

func (r *Round) stateDelete(op *stateOp) {
	ok := r.quorum.Delete(op.id) // TODO: Maybe error instead?
	if !ok {
		op.SetError(fmt.Errorf("coudn't delete Certificate"))
		return
	}
	op.SetError(nil)
}

// AddSignature appends a Signature to one of the [Round]'s Certificates.
func (r *Round) AddSignature(ctx context.Context, id rebro.MessageID, sig crypto.Signature) error {
	op := newStateOp(addSignOp)
	op.id = id
	op.sig = &sig

	return r.execOp(ctx, op)
}

// stateAddSign adds signature to the quorum and attempts to finalize it. If success, it notifies
// all the [Round.Finalize] subscribers.
func (r *Round) stateAddSign(op *stateOp) {
	comm, ok := r.quorum.Get(op.id)
	if !ok {
		op.SetError(fmt.Errorf("coudn't find Certificate"))
		return
	}

	fin, err := comm.AddSignature(*op.sig)
	if err != nil {
		op.SetError(err)
		return
	}
	// check if the certificate is complete
	if !fin {
		op.SetError(nil)
		return
	}
	// check if the quorum has finalized
	ok, err = r.quorum.Finalize()
	if err != nil {
		op.SetError(fmt.Errorf("finalizing quorum certificate: %w", err))
		return
	}
	if !ok {
		op.SetError(nil)
		return
	}
	// ok, it's final, notify everyone
	select {
	case <-r.finalCh:
	default:
		close(r.finalCh)
	}
	op.SetError(nil)
}

// execOp submits operation for execution by [stateLoop] and awaits for its completion
// It permits submission until closedCh is closed or context is cancelled, even after closing is
// triggered. This allows some "last-minute" operations to "squeeze in" before [Round] fully finishes.
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
// and ensures access to QuorumCertificate is single-threaded
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
