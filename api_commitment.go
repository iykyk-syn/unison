package rebro

// Commitment maintains a set of signatures/acknowledgements from a quorum certifying validity
// of an arbitrary broadcasted message. Validity rules are not defined by Commitment and are an external concern.
type Commitment interface {
	// Message returns Message that Commitment attests to.
	Message() Message
	// Signatures provides list of all the signatures in the Commitment.
	Signatures() []Signature
	// AddSignature appends signature of a particular signer to the Commitment.
	// Signature is expected to be verified beforehand.
	// Reports true if enough signatures were collected for complete Commitment.
	AddSignature(Signature) (bool, error)
}

// QuorumCommitment is a set data Commitments(or certificates) by a quorum. It accumulates data
// Commitments propagated over Broadcaster network by quorum participants and maintains *local
// view* of Commitments.
//
// QuorumCommitment is mutable and append-only until its finalized.
// It expects arbitrary number of new Commitments to be added until finalization is triggered.
// The finalization conditions and quorums are implementation specific.
type QuorumCommitment interface {
	// Add constructs new Commitment from the given message and adds it to the set,
	// after checking its round and signer validity.
	Add(Message) error
	// Get retrieves particular Commitment by the MessageID of the committed MessageData.
	Get(MessageID) (Commitment, bool)
	// Delete deletes Commitment by the MessageID of the committed MessageData.
	Delete(MessageID) bool
	// List provides all the non-completed Commitments in the QuorumCommitment.
	List() []Commitment
	// Finalize attempts to finalize the QuorumCommitment.
	// It reports whether the finalization conditions were met.
	// The finalization conditions are defined by the implementation.
	// It may additionally perform expensive computation, like signature aggregation.
	Finalize() (bool, error)
}
