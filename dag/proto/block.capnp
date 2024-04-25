@0xebe99359e631e3a9;

using Go = import "/go.capnp";
$Go.package("block");
$Go.import("block/proto/block");

struct Block {
    round @0 :UInt64;
    signer @1 :Data;
    batches @2 :List(Data);
    parents @3 :List(Data);
}

struct BlockID {
    round @0 :UInt64;
    signer @1 :Data;
    hash @2 :Data;
}