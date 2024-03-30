@0xfbd8d724be65e33e;
using Go = import "/go.capnp";
$Go.package("gossipmsg");
$Go.import("gossip/gossipmsg");

struct Gossip {
    id @0 :Data;
    union {
        signature :group {
            signer @1 :Data;
            signature @2 :Data;
        }
        data :group {
            data @3 :Data;
        }
    }
}