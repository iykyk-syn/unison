package gossip

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
	"github.com/iykyk-syn/unison/rebro/gossip/gossipmsg"
	"github.com/iykyk-syn/unison/rebro/gossip/internal/round"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Broadcaster struct {
	networkID rebro.NetworkID

	rounds *round.Manager
	pubsub *pubsub.PubSub
	topic  *pubsub.Topic
	sub    *pubsub.Subscription

	signer   crypto.Signer
	verifier rebro.Verifier
	hasher   rebro.Hasher
	decoder  rebro.MessageIDDecoder

	log *slog.Logger
}

// NewBroadcaster instantiates a new gossiping [Broadcaster].
func NewBroadcaster(networkID rebro.NetworkID, singer crypto.Signer, verifier rebro.Verifier, hasher rebro.Hasher, decoder rebro.MessageIDDecoder, ps *pubsub.PubSub) *Broadcaster {
	return &Broadcaster{
		networkID: networkID,
		rounds:    round.NewManager(),
		pubsub:    ps,
		signer:    singer,
		verifier:  verifier,
		hasher:    hasher,
		decoder:   decoder,
	}
}

func (bro *Broadcaster) Start() (err error) {
	if bro.log == nil {
		bro.log = slog.Default()
	}

	// TODO(@Wondartan): versioning for topic
	bro.topic, err = bro.pubsub.Join(bro.networkID.String())
	if err != nil {
		return err
	}

	// pubsub forces us to create at least one subscription
	bro.sub, err = bro.topic.Subscribe()
	if err != nil {
		return err
	}
	go func() {
		for {
			_, err := bro.sub.Next(context.Background())
			if err != nil {
				return
			}
		}
	}()

	err = bro.pubsub.RegisterTopicValidator(
		bro.networkID.String(),
		bro.deliverGossip,
		pubsub.WithValidatorTimeout(time.Second),
	)
	if err != nil {
		return err
	}

	return nil
}

func (bro *Broadcaster) Stop(ctx context.Context) (err error) {
	bro.sub.Cancel()
	err = errors.Join(err, bro.topic.Close())
	err = errors.Join(err, bro.pubsub.UnregisterTopicValidator(bro.networkID.String()))
	err = errors.Join(err, bro.rounds.Stop(ctx))
	return err
}

func (bro *Broadcaster) Broadcast(ctx context.Context, msg rebro.Message, qcomm rebro.QuorumCertificate) error {
	r, err := bro.rounds.StartRound(msg.ID.Round(), qcomm)
	if err != nil {
		return err
	}

	err = bro.broadcastGossip(ctx, func(message gossipmsg.Gossip) error {
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

	err = r.Finalize(ctx)
	if err != nil {
		return err
	}

	// TODO: Delayed stopped to collect more signatures
	return bro.rounds.StopRound(ctx, msg.ID.Round())
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
func (bro *Broadcaster) deliverGossip(ctx context.Context, _ peer.ID, gossip *pubsub.Message) (res pubsub.ValidationResult) {
	defer func() {
		// recover from potential panics caused by network gossips
		err := recover()
		if err != nil {
			bro.log.ErrorContext(ctx, "deliver gossip panic", "err", err)
			res = pubsub.ValidationReject
		}
	}()

	msgMsg, err := capnp.Unmarshal(gossip.Data)
	if err != nil {
		bro.log.ErrorContext(ctx, "unmarshalling gossip data", "err", err)
		return pubsub.ValidationReject
	}

	msg, err := gossipmsg.ReadRootGossip(msgMsg)
	if err != nil {
		bro.log.ErrorContext(ctx, "unmarshalling gossip data", "err", err)
		return pubsub.ValidationReject
	}

	err = bro.processGossip(ctx, msg)
	if err != nil {
		bro.log.ErrorContext(ctx, "processing gossip", "err", err)
		return pubsub.ValidationReject
	}

	return pubsub.ValidationAccept
}
