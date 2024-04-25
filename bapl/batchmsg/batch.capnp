@0xae6a3c85d527139f;
using Go = import "/go.capnp";
$Go.package("batchmsg");
$Go.import("bapl/batchmsg");

struct Batch {
    data @0 :Data;
    signature :group {
        signer @1 :Data;
        signature @2 :Data;
    }
}