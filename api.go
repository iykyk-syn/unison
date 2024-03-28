// Package bcast enables:
//   - High throughput censorship resistant commitments
//   - Dynamic and randomized quorums
//   - Customization of hashing functions and signing schemes, including aggregatable signatures.
//   - Customization of broadcasting algorithms and networking stacks
//   - Custom quorum fault parameters and sizes.
//
// A trivial consensus algorithm to implement here would be to:
// * Require full quorum as finalization condition
// * Order blocks by public keys of quorum participants lexicographically
package bcast

import (
	"context"
)

// Message encapsulates data to be broadcasted with a signature of the broadcasting identity.
type Message struct {
	// Signature over Message Data.
	Signature Signature
	// Data of the Message.
	Data []byte
}

// Signature is a tuple containing signature body and reference to signing identity.
type Signature struct {
	// Body of the signature.
	Body []byte
	// Signer identity who produced the signature.
	Signer []byte
}

// QuorumCommitment is a set data Commitments(or certificates) by a quorum. It accumulates data
// Commitments propagated over Broadcaster network by quorum participants and maintains *local
// view* of Commitments.
//
// QuorumCommitment is mutable and append-only until its finalized.
// It expects arbitrary number of new Commitments to be added until finalization is triggered.
// The finalization conditions and quorums are implementation specific.
type QuorumCommitment interface {
	// Add constructs new Commitment from given Message input, adds it to the set and returns.
	// It should do necessary verification of the Message and its signature.
	Add(Message) (Commitment, error)
	// Get retrieves particular Commitment by the hash string of the committed Message.
	Get(messagehash string) (Commitment, bool)
	// Delete deletes Commitment by the hash sting of the committed Message.
	Delete(messagehash string) bool
	// List provides all the Commitments in the QuorumCommitment.
	List() []Commitment
	// Finalize awaits finalization condition of the QuorumCommitment.
	// The finalization condition are defined by the implementation.
	Finalize(context.Context) error
}

// Commitment maintains a set of signatures/acknowledgements from quorum certifying validity
// of an arbitrary Message. Validity rules are not defined by Commitment and are an external concern.
type Commitment interface {
	// Message returns data the Commitment attests to.
	Message() Message
	// MessageHash provides the hash digest of the committed Message.
	MessageHash() string
	// Signatures provides list of all the signatures in the Commitment.
	Signatures() []Signature
	// AddSignature appends signature of a particular signer to the Commitment.
	AddSignature(Signature) error
}

// Broadcaster reliably broadcasts and commits the given Message. It delivers and verifies Messages
// broadcasted by other quorum participants and accumulates them into QuorumCommitment until its
// finalized.
//
// Broadcaster defines interface for asynchronous byzantine reliable quorum broadcast.
// It is responsible for reliable broadcasting and certification of an arbitrary data without
// partial synchrony. It enables parallel quorum commitments and multiple broadcasters can propose
// their Messages simultaneously that other quorum participants attest to.
//
// Broadcaster enables optionality(through polymorphism) for networking algorithms
// (leader-based or mesh-based) by decoupling commitment data structure.
type Broadcaster interface {
	// Broadcast broadcasts given Message and awaits for Message and Signatures from other quorum
	// participants until QuorumCommitment is finalized.
	Broadcast(context.Context, Message, QuorumCommitment) error
}
