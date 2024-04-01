// Package rebro enables:
//   - High throughput censorship resistant commitments
//   - Dynamic and randomized quorums
//   - Customization of hashing functions and signing schemes, including aggregatable signatures.
//   - Customization of broadcasting algorithms and networking stacks
//   - Custom quorum fault parameters and sizes.
//
// A trivial consensus algorithm to implement here would be to:
// * Require full quorum as finalization condition
// * Order blocks by public keys of quorum participants lexicographically
package rebro

import (
	"context"
)

// Message is message to be reliably broadcasted.
type Message struct {
	// ID holds MessageID of the Message.
	ID MessageID
	// Data holds arbitrary bytes data of the message.
	Data []byte
}

// MessageID contains metadata that uniquely identifies a broadcasted message. It specifies
// a minimally required interface all messages should conform to in order to be securely broadcasted.
type MessageID interface {
	// Round returns the monotonically increasing round of the broadcasted message.
	Round() uint64
	// Signer returns identity of the entity committing to the message.
	Signer() []byte
	// Hash returns the hash digest of the message.
	Hash() []byte
	// String returns string representation of the message.
	String() string

	// New instantiates a new MessageID.
	// Required for generic marshalling.
	New() MessageID
	// MarshalBinary serializes MessageID into series of bytes.
	// Must return canonical representation of MessageData
	MarshalBinary() ([]byte, error)
	// UnmarshalBinary deserializes MessageID from a serias of bytes.
	UnmarshalBinary([]byte) error
}

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
	// Quorum back-references QuorumCommitment the Commitment is attached to.
	Quorum() QuorumCommitment
}

// QuorumCommitment is a set data Commitments(or certificates) by a quorum. It accumulates data
// Commitments propagated over Broadcaster network by quorum participants and maintains *local
// view* of Commitments.
//
// QuorumCommitment is mutable and append-only until its finalized.
// It expects arbitrary number of new Commitments to be added until finalization is triggered.
// The finalization conditions and quorums are implementation specific.
type QuorumCommitment interface {
	// Add constructs new Commitment from given the given message and adds it to the set
	// performing necessary verification.
	Add(Message) error
	// Get retrieves particular Commitment by the MessageID of the committed MessageData.
	Get(MessageID) (Commitment, bool)
	// Delete deletes Commitment by the MessageID of the committed MessageData.
	Delete(MessageID) bool
	// List provides all the Commitments in the QuorumCommitment.
	List() []Commitment
	// Finalize attempts to finalize the QuorumCommitment.
	// It reports whether the finalization conditions were met.
	// The finalization conditions are defined by the implementation.
	// It may additionally perform expensive computation, like signature aggregation.
	Finalize() (bool, error)
}

// Broadcaster reliably broadcasts, delivers and commits over messages. It verifies Messages
// delivered from other quorum participants and accumulates them into QuorumCommitment until its
// finalized.
//
// Broadcaster defines interface for asynchronous byzantine reliable quorum broadcast.
// It is responsible for reliable broadcasting and certification of an arbitrary data without
// partial synchrony. It enables parallel quorum commitments and multiple broadcasters can propose
// their Messages simultaneously that other quorum participants attest to.
//
// Broadcaster enables optionality(through polymorphism) for networking algorithms
// (leader-based or mesh-based) by decoupling commitment data structure.
//
// It signs over broadcasted MessageIDs automatically after verifying them using Signer.
// TODO: Explain rules around rounds
type Broadcaster interface {
	// Broadcast broadcasts and delivers messages from quorum participants and signatures over them
	// until QuorumCommitment is finalized.
	Broadcast(context.Context, Message, QuorumCommitment) error
}

// Verifier performs application specific message stateful verification.
// It used by Broadcaster during broadcasting rounds.
type Verifier interface {
	// Verify executes verification of every Message delivered to QuorumCommitment
	// within a broadcasting round.
	// Message is guaranteed to be valid by the rules in QuorumCommitment.
	Verify(context.Context, Message) error
}

// Hasher hashes Messages to cross-check their validity with MessageID.Hash
type Hasher interface {
	Hash(Message) ([]byte, error)
}

// NetworkID identifies a particular network of nodes.
type NetworkID string

// String returns string representation of NetworkID.
func (nid NetworkID) String() string {
	return string(nid)
}

// Orchestrator orchestrates multiple Broadcaster instances.
type Orchestrator interface {
	// NewBroadcaster instantiates a new Broadcaster.
	NewBroadcaster(NetworkID, Signer, Verifier, Hasher) (Broadcaster, error)
}
