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
	NewBroadcaster(NetworkID, Signer, Verifier, Hasher, MessageIDDecoder) (Broadcaster, error)
}
