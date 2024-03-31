package round

import (
	"sync"

	"github.com/1ykyk/rebro"
)

// stateOpPool pools allocated stateOps for later reuse
// this reduce GC pressure
var stateOpPool = sync.Pool{New: func() any {
	return &stateOp{
		doneCh: make(chan any, 1),
	}
}}

// defines types of state machine operations
type stateOpKind uint8

const (
	addOp stateOpKind = iota
	getOp
	deleteOp
	addSignOp
)

// stateOp defines operations on the Round state machine
type stateOp struct {
	kind   stateOpKind
	doneCh chan any

	// request data:
	msg *rebro.Message   // addOp
	id  rebro.MessageID  // getOp or deleteOp
	sig *rebro.Signature // addSignOp

	// response data:
	err  error            // addOp, deleteOp, addSignOp
	comm rebro.Commitment // getOp
}

func newStateOp(kind stateOpKind) *stateOp {
	op := stateOpPool.Get().(*stateOp)
	op.kind = kind
	return op
}

// Free frees up the op for reuse.
func (op *stateOp) Free() {
	// empty all the fields for the next user
	// but doneCh is reusable
	op.msg = nil
	op.id = nil
	op.sig = nil
	op.err = nil
	op.comm = nil
	stateOpPool.Put(op)
}

// SetCommitment sets commitment result on the operation
// and notifies that operation has been done.
func (op *stateOp) SetCommitment(comm rebro.Commitment) {
	op.comm = comm
	op.doneCh <- comm
}

// SetError sets error result on the operation
// and notifies that operation has been done.
func (op *stateOp) SetError(err error) {
	op.err = err
	op.doneCh <- err
}
