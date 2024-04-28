package block

import (
	"crypto/sha256"
	"fmt"

	"capnproto.org/go/capnp/v3"
	block "github.com/iykyk-syn/unison/dag/block/blockmsg"
	"github.com/iykyk-syn/unison/rebro"
)

type blockID struct {
	round  uint64
	signer []byte
	hash   []byte
}

func UnmarshalBlockID(bytes []byte) (rebro.MessageID, error) {
	id := blockID{}
	return &id, id.UnmarshalBinary(bytes)
}

func (id *blockID) Round() uint64 {
	return id.round
}

func (id *blockID) Signer() []byte {
	return id.signer
}

func (id *blockID) Hash() []byte {
	return id.hash
}

func (id *blockID) String() string {
	return fmt.Sprintf("%X", id.hash)
}

func (id *blockID) MarshalBinary() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("creating a segemnt for capnp:%v", err)
	}

	blockId, err := block.NewRootBlockID(seg)
	if err != nil {
		return nil, fmt.Errorf("converting segment to message id:%v", err)
	}

	err = blockId.SetHash(id.hash)
	if err != nil {
		return nil, err
	}
	blockId.SetRound(id.round)
	err = blockId.SetSigner(id.signer)
	if err != nil {
		return nil, err
	}
	return msg.Marshal()
}

func (id *blockID) UnmarshalBinary(data []byte) error {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return err
	}

	msgID, err := block.ReadRootBlockID(msg)
	if err != nil {
		return fmt.Errorf("converting received binary data to messageID: %v", err)
	}

	id.round = msgID.Round()
	id.hash, err = msgID.Hash()
	if err != nil {
		return err
	}
	id.signer, err = msgID.Signer()
	return err
}

func (id *blockID) Validate() error {
	if len(id.hash) != sha256.Size {
		return fmt.Errorf("invalid hash")
	}
	// TODO: Add more validation
	return nil
}
