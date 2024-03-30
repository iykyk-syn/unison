@0xd5d5bfc56971e700;

using Go = import "/go.capnp";
$Go.package("message_id");
$Go.import("message");

struct MessageID {
    round @0 :UInt64;
    signer @1 :Data;
    hash @2 :Data;
}