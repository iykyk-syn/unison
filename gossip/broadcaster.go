package gossip

import (
	"context"
	"errors"

	"github.com/1ykyk/rebro"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type Broadcaster struct {
	networkID rebro.NetworkID

	pubsub *pubsub.PubSub
	topic  *pubsub.Topic
	sub    *pubsub.Subscription

	signer   rebro.Signer
	verifier rebro.Verifier
}

// NewBroadcaster instantiates a new gossip Broadcaster.
//
// It requires Signer to signMessage/produce certificates automatically.
func NewBroadcaster(networkID rebro.NetworkID, singer rebro.Signer, verifier rebro.Verifier, ps *pubsub.PubSub) (*Broadcaster, error) {
	// TODO(@Wondartan): versioning for topic
	topic, err := ps.Join(networkID.String())
	if err != nil {
		return nil, err
	}

	// pubsub forces us to create at least one subscription
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			_, err := sub.Next(context.Background())
			if err != nil {
				return
			}
		}
	}()

	bro := &Broadcaster{
		networkID: networkID,
		pubsub:    ps,
		topic:     topic,
		signer:    singer,
		verifier:  verifier,
	}

	err = ps.RegisterTopicValidator(networkID.String(), bro.deliverMessage)
	if err != nil {
		return nil, err
	}

	return bro, nil
}

func (bro *Broadcaster) Close() (err error) {
	bro.sub.Cancel()
	err = errors.Join(bro.topic.Close())
	err = errors.Join(bro.pubsub.UnregisterTopicValidator(bro.networkID.String()))
	return err
}

func (bro *Broadcaster) Broadcast(ctx context.Context, msg rebro.Message, qcomm rebro.QuorumCommitment) error {
	err := bro.broadcastMessage(ctx, msg.ID, msg.Data)
	if err != nil {
		return err
	}

	if err := qcomm.Finalize(ctx); err != nil {
		return err
	}

	return nil
}
