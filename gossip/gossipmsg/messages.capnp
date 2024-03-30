@0xfbd8d724be65e33e;
using Go = import "/go.capnp";
$Go.package("gossipmsg");
$Go.import("gossip/gossipmsg");

struct Message {
    id @0 :Data;
    signer @1 :Data;
    signature @2 :Data;
    union {
        data @3 :Data;
        noData @4 :Void;
    }
}