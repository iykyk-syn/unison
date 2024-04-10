package rebro

// Signature is a tuple containing signature body and reference to signing identity.
type Signature struct {
	// Body of the signature.
	Body []byte
	// Signer identity who produced the signature.
	Signer []byte
}

// Signer encapsulates and separates asymmetric cryptographic schema out of Broadcasting protocol
// logic together with private key management.
type Signer interface {
	// ID returns Signer identity like public key
	ID() []byte
	// Sign produces a cryptographic Signature over the given data with internally managed identity.
	Sign([]byte) (Signature, error)
	// Verify performs cryptographic Signature verification of the given data.
	// TODO: Probably signer should verify own signature, and we should a have separate function `verify` to validate
	// the signatures from other users
	Verify([]byte, Signature) error
}
