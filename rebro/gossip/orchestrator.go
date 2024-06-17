package gossip

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

type Orchestrator struct {
	pubsub *pubsub.PubSub
}

func NewOrchestrator(ps *pubsub.PubSub) *Orchestrator {
	return &Orchestrator{pubsub: ps}
}

func (o *Orchestrator) NewBroadcaster(
	nid rebro.NetworkID,
	signer crypto.Signer,
	certifier rebro.Certifier,
	hasher rebro.Hasher,
	decoder rebro.MessageIDDecoder,
) (rebro.Broadcaster, error) {
	bro := NewBroadcaster(nid, signer, certifier, hasher, decoder, o.pubsub)
	return bro, bro.Start()
}
