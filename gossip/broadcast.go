package gossip

import (
	"bytes"
	"context"
	"fmt"

	"capnproto.org/go/capnp/v3"
	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/gossip/gossipmsg"
)

// broadcastMessage prepares, signs and publishes the message to the network.
func (bro *Broadcaster) broadcastMessage(ctx context.Context, id rebro.MessageID, data []byte) error {
	msgMsg, msgSegment, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return err
	}

	msg, err := gossipmsg.NewRootMessage(msgSegment)
	if err != nil {
		return err
	}

	if err = bro.signMessage(id, msg); err != nil {
		return err
	}

	msg.SetNoData()
	if data != nil {
		if err = msg.SetData(data); err != nil {
			return err
		}
		// omit signer, the assumption here that ID already contains it,
		// and we don't need to repeat it
		if err = msg.SetSigner(nil); err != nil {
			return err
		}
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

// signMessage signs and populates the message.
func (bro *Broadcaster) signMessage(id rebro.MessageID, msg gossipmsg.Message) error {
	canonicalID, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	signature, err := bro.signer.Sign(canonicalID)
	if err != nil {
		return err
	}

	if !bytes.Equal(id.Signer(), signature.Signer) {
		// TODO: Append more info to the error
		return fmt.Errorf("inconsistent Signers in the MessageData and Brodcast")
	}

	if err = msg.SetId(canonicalID); err != nil {
		return err
	}

	if err = msg.SetSignature(signature.Body); err != nil {
		return err
	}

	if err = msg.SetSigner(signature.Signer); err != nil {
		return err
	}

	return nil
}
