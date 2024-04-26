package rebro

import "github.com/iykyk-syn/unison/crypto"

// Certificate maintains a set of signatures/acknowledgements from a quorum certifying validity
// of an arbitrary broadcasted message. Validity rules are not defined by Certificate and are an external concern.
type Certificate interface {
	// Message returns Message that Certificate attests to.
	Message() Message
	// Signatures provides list of all the signatures in the Certificate.
	Signatures() []crypto.Signature
	// AddSignature appends signature of a particular signer to the Certificate.
	// Signature is expected to be verified beforehand.
	// Reports true if enough signatures were collected for complete Certificate.
	AddSignature(crypto.Signature) (bool, error)
}

// QuorumCertificate is a set data Certificates by a quorum. It accumulates data
// Certificates propagated over Broadcaster network by quorum participants and maintains *local
// view* of Certificates.
//
// QuorumCertificate is mutable and append-only until its finalized.
// It expects arbitrary number of new Certificates to be added until finalization is triggered.
// The finalization conditions and quorums are implementation specific.
type QuorumCertificate interface {
	// Add constructs new Certificate from given the given message and adds it to the set
	// performing necessary verification.
	Add(Message) error
	// Get retrieves particular Certificate by the MessageID of the committed MessageData.
	Get(MessageID) (Certificate, bool)
	// Delete deletes Certificate by the MessageID of the committed MessageData.
	Delete(MessageID) bool
	// List provides all completed Certificates in the QuorumCertificate.
	List() []Certificate
	// Finalize attempts to finalize the QuorumCertificate.
	// It reports whether the finalization conditions were met.
	// The finalization conditions are defined by the implementation.
	// It may additionally perform expensive computation, like signature aggregation.
	Finalize() (bool, error)
}
