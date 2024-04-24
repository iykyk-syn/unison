@0xebe99359e631e3a9;

using Go = import "/go.capnp";
$Go.package("block_id");
$Go.import("block/proto/block_id");

struct BlockID {
    round @0 :UInt64;
    signer @1 :Data;
    hash @2 :Data;
}