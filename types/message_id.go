package types

import (
	"errors"
	"fmt"

	"capnproto.org/go/capnp/v3"

	message_id "github.com/1ykyk/rebro/types/capnproto"
)

// messageID implements `MessageID` interface and contains metadata for the underlying data.
type messageID struct {
	// round holds the number corresponding to a specific iteration to which data belongs.
	round uint64
	// signer holds a producer of the message.
	signer []byte
	// hash holds the data hash.
	hash []byte
}

func (m *messageID) Round() uint64 {
	return m.round
}

func (m *messageID) Signer() []byte {
	return m.signer
}

func (m *messageID) Hash() []byte {
	return m.hash
}

func (m *messageID) String() string {
	return fmt.Sprintf("Hash{%v}", string(m.hash))
}

func (m *messageID) MarshalBinary() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("creating a segemnt for capnp:%v", err)
	}

	msgId, err := message_id.NewMessageID(seg)
	if err != nil {
		return nil, fmt.Errorf("converting segment to message id:%v", err)
	}

	msgId.SetHash(m.hash)
	msgId.SetRound(m.round)
	msgId.SetSigner(m.signer)

	return msg.Marshal()
}

// UnmarshalBinary deserializes MessageID from a serias of bytes.
func (m *messageID) UnmarshalBinary(data []byte) error {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return err
	}

	msgID, err := message_id.ReadRootMessageID(msg)
	if err != nil {
		return fmt.Errorf("converting received binary data to messageID: %v", err)
	}

	m.round = msgID.Round()
	m.hash, err = msgID.Hash()
	if err != nil {
		return err
	}
	m.signer, err = msgID.Signer()
	return m.ValidateBasic()
}

func (m *messageID) ValidateBasic() error {
	if m.round == 0 {
		return errors.New("round was not set")
	}
	if len(m.hash) == 0 {
		return errors.New("empty hash")
	}
	if len(m.signer) == 0 {
		return errors.New("empty signer")
	}
	return nil
}
