package gossip

import (
	"context"

	"capnproto.org/go/capnp/v3"
	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/gossip/gossipmsg"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// deliverMessage delivers a PubSub gossip and reports its validity status
func (bro *Broadcaster) deliverMessage(ctx context.Context, _ peer.ID, gossip *pubsub.Message) pubsub.ValidationResult {
	msgMsg, err := capnp.Unmarshal(gossip.Data)
	if err != nil {
		return pubsub.ValidationReject
	}

	msg, err := gossipmsg.ReadRootMessage(msgMsg)
	if err != nil {
		return pubsub.ValidationReject
	}

	err = bro.processMessage(ctx, msg)
	if err != nil {
		return pubsub.ValidationReject
	}

	return pubsub.ValidationAccept
}

// processMessage processes and verifies incoming messages filling up QuorumCommitment until its
// finalized.
//
// It verifies any message step-by-step in a particular order of checks:
// * is at valid round and associated with local QuorumCommitment on the same round
// * valid by the rules of the QuorumCommitment
// * contains a valid signature
// * verified by Verifier(if data message)
func (bro *Broadcaster) processMessage(ctx context.Context, msg gossipmsg.Message) (err error) {
	// make sure we have a commitment associated with the message
	comm, err := bro.prepareCommitment(ctx, msg)
	if err != nil {
		return err
	}

	if msg.HasData() {
		defer func() {
			if err != nil {
				// if it is a data message, and we got an error here
				// it means something is wrong with the message and thus its commitment,
				// so delete it
				ok := comm.Quorum().Delete(comm.Message().ID)
				if !ok {
					// TODO Log
				}
			}
		}()
	}

	err = bro.verifySignature(msg, comm)
	if err != nil {
		return err
	}

	if msg.HasData() {
		// verify only once when we got a message with data
		if err = bro.verifier.Verify(ctx, comm.Message()); err != nil {
			return err
		}

		bro.broadcastMessage(ctx, comm.Message().ID, nil)
	}

	return nil
}

// prepareCommitment creates a new commitment if the massage contains data. If it is not a data message,
// the method gets the commitment locally or waits until its created.
func (bro *Broadcaster) prepareCommitment(ctx context.Context, msg gossipmsg.Message) (rebro.Commitment, error) {
	idData, err := msg.Id()
	if err != nil {
		return nil, err
	}

	var id rebro.MessageID
	err = id.New().UnmarshalBinary(idData)
	if err != nil {
		return nil, err
	}

	// TODO: Must have a timeout on it. If not found in time,
	//  it might be either malicious or stale message. The allocated resources(routine and memory)
	//  will be cleaned up by GC. There must a limit for the amount of awaiting getCommitments
	qcomm, err := bro.findQuorumCommitment(ctx, id.Round())
	if err != nil {
		return nil, err
	}

	if msg.HasData() {
		data, err := msg.Data()
		if err != nil {
			return nil, err
		}

		// we add message before signature verification to avoid unnecessary compute
		err = qcomm.Add(rebro.Message{ID: id, Data: data})
		if err != nil {
			return nil, err
		}

		// TODO: unlock all the waiters
	}

	comm, ok := qcomm.Get(id)
	if !ok {
		// TODO: block and wait until commitment

	}

	return comm, nil
}

// verifySignature verifies signature in the message and adds it to the commitment, if correct.
func (bro *Broadcaster) verifySignature(msg gossipmsg.Message, comm rebro.Commitment) error {
	if msg.HasData() {
		// we omit signer field over the wire in case its data message to avoid duplication
		// set the omitted signer back
		if err := msg.SetSigner(comm.Message().ID.Signer()); err != nil {
			return err
		}
	}

	signatureData, err := msg.Signature()
	if err != nil {
		return err
	}

	signerData, err := msg.Signer()
	if err != nil {
		return err
	}

	signature := rebro.Signature{
		Body:   signatureData,
		Signer: signerData,
	}

	idData, err := msg.Id()
	if err != nil {
		return err
	}

	if err := bro.signer.Verify(idData, signature); err != nil {
		return err
	}

	_, err = comm.AddSignature(signature)
	if err != nil {
		return err
	}

	return nil
}

func (bro *Broadcaster) findQuorumCommitment(ctx context.Context, round uint64) (rebro.QuorumCommitment, error) {
	return nil, nil
}
