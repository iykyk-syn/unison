package dag

import (
	"crypto/sha256"
	"fmt"

	"capnproto.org/go/capnp/v3"

	"github.com/iykyk-syn/unison/bapl"
	block "github.com/iykyk-syn/unison/dag/proto"
)

type Block struct {
	blockID *blockID
	batches [][]byte // hashes of all local batches that will be included in the block
	parents [][]byte // hashes of the blocks from prev round
}

func NewBlock(
	round uint64,
	singer []byte,
	batches []*bapl.Batch,
	parents [][]byte,
) *Block {
	hashes := make([][]byte, len(batches))
	for i := range batches {
		hashes[i] = batches[i].Hash()
	}

	id := &blockID{round: round, signer: singer}
	return &Block{blockID: id, batches: hashes, parents: parents}
}

func (b *Block) Hash() []byte {
	if b.blockID.hash != nil {
		return b.blockID.hash
	}

	bin, err := b.MarshalBinary()
	if err != nil {
		panic(err)
	}
	h := sha256.New()
	h.Write(bin)
	b.blockID.hash = h.Sum(nil)
	return b.blockID.hash
}

func (b *Block) Round() uint64 {
	return b.blockID.round
}

func (b *Block) Signer() []byte {
	return b.blockID.hash
}

func (b *Block) String() string {
	return fmt.Sprintf("%T", b.Hash())
}

func (b *Block) MarshalBinary() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("creating a segemnt for capnp:%v", err)
	}

	block, err := block.NewBlock(seg)
	if err != nil {
		return nil, fmt.Errorf("converting segment to message id:%v", err)
	}

	block.SetRound(b.blockID.round)
	block.SetSigner(b.blockID.signer)
	bList, err := block.NewBatches(int32(len(b.batches)))
	if err != nil {
		return nil, err
	}

	for i, batch := range b.batches {
		bList.Set(i, batch)
	}
	block.SetBatches(bList)

	pList, err := block.NewParents(int32(len(b.parents)))
	for i, pp := range b.parents {
		pList.Set(i, pp)
	}
	block.SetParents(pList)
	return msg.Marshal()
}

func (b *Block) UnmarshalBinary(data []byte) error {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return err
	}

	block, err := block.ReadRootBlock(msg)
	if err != nil {
		return fmt.Errorf("converting received binary data to messageID: %v", err)
	}

	b.blockID.round = block.Round()
	b.blockID.signer, err = block.Signer()
	batchList, err := block.Batches()
	if err != nil {
		return err
	}

	batches := make([][]byte, batchList.Len())
	for i := range batches {
		data, err := batchList.At(i)
		if err != nil {
			return err
		}
		batches[i] = data
	}

	parentsList, err := block.Parents()
	if err != nil {
		return err
	}

	parents := make([][]byte, parentsList.Len())
	for i := range parents {
		data, err := parentsList.At(i)
		if err != nil {
			return err
		}
		parents[i] = data
	}

	b.batches = batches
	b.parents = parents
	return err
}

func (b *Block) Validate() error {
	return nil
}
