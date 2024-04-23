package round

import (
	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

// defines types of state machine operations
type stateOpKind uint8

const (
	addOp stateOpKind = iota
	getOp
	deleteOp
	addSignOp
)

// stateOp defines operations on the [Round] state machine
type stateOp struct {
	kind   stateOpKind
	doneCh chan any

	// request data:
	msg *rebro.Message    // addOp
	id  rebro.MessageID   // getOp or deleteOp
	sig *crypto.Signature // addSignOp

	// response data:
	err  error             // addOp, deleteOp, addSignOp
	comm rebro.Certificate // getOp
}

func newStateOp(kind stateOpKind) *stateOp {
	return &stateOp{kind: kind, doneCh: make(chan any, 1)}
}

// SetCertificate sets [rebro.Certificate] result on the operation
// and notifies that operation has been done.
func (op *stateOp) SetCertificate(comm rebro.Certificate) {
	op.comm = comm
	op.doneCh <- comm
}

// SetError sets error result on the operation
// and notifies that operation has been done.
func (op *stateOp) SetError(err error) {
	op.err = err
	op.doneCh <- err
}
