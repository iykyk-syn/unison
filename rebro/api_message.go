package rebro

import "errors"

// Message is message to be reliably broadcasted.
type Message struct {
	// ID holds MessageID of the Message.
	ID MessageID
	// Data holds arbitrary bytes data of the message.
	Data []byte
}

func (m *Message) Validate() error {
	if len(m.Data) == 0 || m.ID == nil {
		return errors.New("empty data or id of the message")
	}
	return m.ID.Validate()
}

// MessageID contains metadata that uniquely identifies a broadcasted message. It specifies
// a minimally required canonical interface all messages should conform to in order to be securely broadcasted.
type MessageID interface {
	// Round returns the monotonically increasing round of the broadcasted message.
	Round() uint64
	// Signer returns identity of the entity committing to the message.
	Signer() []byte
	// Hash returns the hash digest of the message.
	Hash() []byte
	// String returns string representation of the message.
	String() string
	// MarshalBinary serializes MessageID into series of bytes.
	// Must return canonical representation of MessageData
	MarshalBinary() ([]byte, error)
	// UnmarshalBinary deserializes MessageID from a series of bytes.
	UnmarshalBinary([]byte) error
	// Validate performs the basic validation messageID's properties before it will be used across the protocol.
	Validate() error
}

// MessageIDDecoder unmarshalls Messages of a particular type.
type MessageIDDecoder func([]byte) (MessageID, error)
