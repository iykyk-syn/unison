package dag

import (
	"fmt"

	"capnproto.org/go/capnp/v3"

	"github.com/iykyk-syn/unison/bapl"
	block_id "github.com/iykyk-syn/unison/dag/proto"
)

type Block struct {
	id            *blockID
	BatchesDigest [][]byte // hashes of all local batches that will be included in the block
	Parents       [][]byte // hashes of the blocks from prev round
}

func NewBlock(id *blockID, batches []*bapl.Batch, parents [][]byte) *Block {
	hashes := make([][]byte, len(batches))
	for i := range batches {
		hashes[i] = batches[i].Hash()
	}
	return &Block{id: id, BatchesDigest: hashes, Parents: parents}
}

type blockID struct {
	round  uint64
	signer []byte
	hash   []byte
}

func (b *Block) ID() *blockID {
	return b.id
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
	return fmt.Sprintf("%T", id.hash)
}

func (id *blockID) MarshalBinary() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("creating a segemnt for capnp:%v", err)
	}

	blockId, err := block_id.NewBlockID(seg)
	if err != nil {
		return nil, fmt.Errorf("converting segment to message id:%v", err)
	}

	blockId.SetHash(id.hash)
	blockId.SetRound(id.round)
	blockId.SetSigner(id.signer)
	return msg.Marshal()
}

func (id *blockID) UnmarshalBinary(data []byte) error {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return err
	}

	msgID, err := block_id.ReadRootBlockID(msg)
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
	return nil
}
