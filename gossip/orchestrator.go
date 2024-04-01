package gossip

import (
	"github.com/1ykyk/rebro"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type Orchestrator struct {
	pubsub *pubsub.PubSub
}

func NewOrchestrator(ps *pubsub.PubSub) *Orchestrator {
	return &Orchestrator{pubsub: ps}
}

func (o *Orchestrator) NewBroadcaster(nid rebro.NetworkID, signer rebro.Signer, verifier rebro.Verifier, hasher rebro.Hasher) (rebro.Broadcaster, error) {
	bro := NewBroadcaster(nid, signer, verifier, hasher, o.pubsub)
	return bro, bro.Start()
}
