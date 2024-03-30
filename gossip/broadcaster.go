package gossip

import (
	"context"
	"errors"
	"hash"

	"capnproto.org/go/capnp/v3"
	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/gossip/gossipmsg"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TODO:
//  * Pubsubbing for quorum commitment
//  * Add logging
//  * Add metrics

type Broadcaster struct {
	networkID rebro.NetworkID

	pubsub *pubsub.PubSub
	topic  *pubsub.Topic
	sub    *pubsub.Subscription

	signer   rebro.Signer
	verifier rebro.Verifier
	hash     func() hash.Hash
}

// NewBroadcaster instantiates a new gossip Broadcaster.
//
// It requires Signer to signMessage/produce certificates automatically.
func NewBroadcaster(networkID rebro.NetworkID, singer rebro.Signer, verifier rebro.Verifier, hash func() hash.Hash, ps *pubsub.PubSub) (*Broadcaster, error) {
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
		hash:      hash,
	}

	err = ps.RegisterTopicValidator(networkID.String(), bro.deliverGossip)
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
	err := bro.broadcastGossip(ctx, func(message gossipmsg.Gossip) error {
		canonicalID, err := msg.ID.MarshalBinary()
		if err != nil {
			return err
		}
		if err = message.SetId(canonicalID); err != nil {
			return err
		}
		if err = message.Data().SetData(msg.Data); err != nil {
			return err
		}
		message.SetData()
		return nil
	})
	if err != nil {
		return err
	}

	if err := qcomm.Finalize(ctx); err != nil {
		return err
	}

	return nil
}

// broadcastGossip prepares and publishes a gossip to the network.
func (bro *Broadcaster) broadcastGossip(ctx context.Context, setter func(gossipmsg.Gossip) error) error {
	msgMsg, msgSegment, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return err
	}

	msg, err := gossipmsg.NewRootGossip(msgSegment)
	if err != nil {
		return err
	}

	if err = setter(msg); err != nil {
		return err
	}

	bytes, err := msgMsg.Marshal()
	if err != nil {
		return err
	}

	err = bro.topic.Publish(ctx, bytes)
	if err != nil {
		return err
	}

	return nil
}

// deliverGossip delivers a PubSub gossip and reports its validity status
func (bro *Broadcaster) deliverGossip(ctx context.Context, _ peer.ID, gossip *pubsub.Message) pubsub.ValidationResult {
	// TODO: Catch panics
	msgMsg, err := capnp.Unmarshal(gossip.Data)
	if err != nil {
		return pubsub.ValidationReject
	}

	msg, err := gossipmsg.ReadRootGossip(msgMsg)
	if err != nil {
		return pubsub.ValidationReject
	}

	err = bro.processGossip(ctx, msg)
	if err != nil {
		return pubsub.ValidationReject
	}

	return pubsub.ValidationAccept
}
