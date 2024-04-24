// Code generated by capnpc-go. DO NOT EDIT.

package block_id

import (
	capnp "capnproto.org/go/capnp/v3"
	text "capnproto.org/go/capnp/v3/encoding/text"
	schemas "capnproto.org/go/capnp/v3/schemas"
)

type BlockID capnp.Struct

// BlockID_TypeID is the unique identifier for the type BlockID.
const BlockID_TypeID = 0xe8be3e5a06a33e53

func NewBlockID(s *capnp.Segment) (BlockID, error) {
	st, err := capnp.NewStruct(s, capnp.ObjectSize{DataSize: 8, PointerCount: 2})
	return BlockID(st), err
}

func NewRootBlockID(s *capnp.Segment) (BlockID, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 8, PointerCount: 2})
	return BlockID(st), err
}

func ReadRootBlockID(msg *capnp.Message) (BlockID, error) {
	root, err := msg.Root()
	return BlockID(root.Struct()), err
}

func (s BlockID) String() string {
	str, _ := text.Marshal(0xe8be3e5a06a33e53, capnp.Struct(s))
	return str
}

func (s BlockID) EncodeAsPtr(seg *capnp.Segment) capnp.Ptr {
	return capnp.Struct(s).EncodeAsPtr(seg)
}

func (BlockID) DecodeFromPtr(p capnp.Ptr) BlockID {
	return BlockID(capnp.Struct{}.DecodeFromPtr(p))
}

func (s BlockID) ToPtr() capnp.Ptr {
	return capnp.Struct(s).ToPtr()
}
func (s BlockID) IsValid() bool {
	return capnp.Struct(s).IsValid()
}

func (s BlockID) Message() *capnp.Message {
	return capnp.Struct(s).Message()
}

func (s BlockID) Segment() *capnp.Segment {
	return capnp.Struct(s).Segment()
}
func (s BlockID) Round() uint64 {
	return capnp.Struct(s).Uint64(0)
}

func (s BlockID) SetRound(v uint64) {
	capnp.Struct(s).SetUint64(0, v)
}

func (s BlockID) Signer() ([]byte, error) {
	p, err := capnp.Struct(s).Ptr(0)
	return []byte(p.Data()), err
}

func (s BlockID) HasSigner() bool {
	return capnp.Struct(s).HasPtr(0)
}

func (s BlockID) SetSigner(v []byte) error {
	return capnp.Struct(s).SetData(0, v)
}

func (s BlockID) Hash() ([]byte, error) {
	p, err := capnp.Struct(s).Ptr(1)
	return []byte(p.Data()), err
}

func (s BlockID) HasHash() bool {
	return capnp.Struct(s).HasPtr(1)
}

func (s BlockID) SetHash(v []byte) error {
	return capnp.Struct(s).SetData(1, v)
}

// BlockID_List is a list of BlockID.
type BlockID_List = capnp.StructList[BlockID]

// NewBlockID creates a new list of BlockID.
func NewBlockID_List(s *capnp.Segment, sz int32) (BlockID_List, error) {
	l, err := capnp.NewCompositeList(s, capnp.ObjectSize{DataSize: 8, PointerCount: 2}, sz)
	return capnp.StructList[BlockID](l), err
}

// BlockID_Future is a wrapper for a BlockID promised by a client call.
type BlockID_Future struct{ *capnp.Future }

func (f BlockID_Future) Struct() (BlockID, error) {
	p, err := f.Future.Ptr()
	return BlockID(p.Struct()), err
}

const schema_ebe99359e631e3a9 = "x\xda4\xc81J\xc4P\x14\x05\xd0{\xdf\xcf\xcc0" +
	"`\xd0\x0f\xa9\xb4\xb0\x17t\x88\xe5\x14c\x90\x08\x06\x14" +
	"\xf2\xd0F\x1b\x89\x89\x98\xa0\xe4\xc7DW\xe2\x12\xac\xac" +
	"\xdc\x81V.\xc0\xc2\x15\x08\xa2\xb8\x88H\x04\xcbsV" +
	"n\"/\xf4_\x08\xd1`4\xee\x8f\x16\x0f\xe3\xd3\xc5" +
	"\xf3\x17t\x8d\xec\x1f?\xc2\xcf\x93\xfb\xef\x1f\x8cd\x02" +
	"\x84\xaf\xab\xb4\xef\x13\xc0\xbe=a\xb3?\xbfv\xf9\xd5" +
	"\xaci\x8d\xbbu\xb3?\x9cU\xc5V\x9e5u3\xdf" +
	"\x1dh\x928%u\xc9x\x80G\xc0\xeem\x03\x1a\x19" +
	"\xea\x81\x90\x0c8\\2\x0746\xd4Th\x85\x01\x05" +
	"\xb0\x87\x1b\x80\xee\x1b\xea\xb1p\xbduwu\xc1)\x84" +
	"Sp\xa7\xab.\xeb\x8b\x96>\x84>\xb8\\f]\xf9" +
	"\x8f\xdf\x00\x00\x00\xff\xffK[,\x81"

func RegisterSchema(reg *schemas.Registry) {
	reg.Register(&schemas.Schema{
		String: schema_ebe99359e631e3a9,
		Nodes: []uint64{
			0xe8be3e5a06a33e53,
		},
		Compressed: true,
	})
}