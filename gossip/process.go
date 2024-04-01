package gossip

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/gossip/gossipmsg"
)

func (bro *Broadcaster) processGossip(ctx context.Context, gsp gossipmsg.Gossip) error {
	// TODO: DOS protection idea:
	//  * Ensure only N processGossip routines can exist
	//  * Cancel the oldest routine, if a new one does not fit
	//  * Ensure there is a timeout which routine

	switch gsp.Which() {
	case gossipmsg.Gossip_Which_data:
		return bro.processData(ctx, gsp)
	case gossipmsg.Gossip_Which_signature:
		return bro.processSignature(ctx, gsp)
	default:
		return fmt.Errorf("unknown message type")
	}
}

func (bro *Broadcaster) processData(ctx context.Context, gsp gossipmsg.Gossip) error {
	canonicalID, err := gsp.Id()
	if err != nil {
		return err
	}

	data, err := gsp.Data().Data()
	if err != nil {
		return err
	}

	var id rebro.MessageID
	msg := rebro.Message{
		ID:   id.New(),
		Data: data,
	}
	err = msg.ID.UnmarshalBinary(canonicalID)
	if err != nil {
		return err
	}

	hash, err := bro.hasher.Hash(msg)
	if err != nil {
		return err
	}

	if !bytes.Equal(hash, msg.ID.Hash()) {
		return fmt.Errorf("inconsistent message hash")
	}

	r, err := bro.rounds.GetRound(ctx, id.Round())
	if err != nil {
		return err
	}
	// add to quorum and prepare the commitment
	err = r.AddCommitment(ctx, msg)
	if err != nil {
		return err
	}

	if err = bro.verifier.Verify(ctx, msg); err != nil {
		// it means something is wrong with the message and thus its commitment,
		// so delete it
		deleteErr := r.DeleteCommitment(ctx, id)
		if err != nil {
			err = errors.Join(err, deleteErr)
			// TODO Log
		}
		return err
	}

	signature, err := bro.signer.Sign(canonicalID)
	if err != nil {
		return err
	}

	// TODO: Investigate reuse of the message instead of making a new one
	err = bro.broadcastGossip(ctx, func(gsp gossipmsg.Gossip) error {
		gsp.SetSignature()
		if err := gsp.SetId(canonicalID); err != nil {
			return err
		}
		if err := gsp.Signature().SetSignature(signature.Body); err != nil {
			return err
		}
		if err := gsp.Signature().SetSigner(signature.Signer); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (bro *Broadcaster) processSignature(ctx context.Context, gsp gossipmsg.Gossip) error {
	canonicalID, err := gsp.Id()
	if err != nil {
		return err
	}

	var id rebro.MessageID
	id = id.New()
	err = id.UnmarshalBinary(canonicalID)
	if err != nil {
		return err
	}

	r, err := bro.rounds.GetRound(ctx, id.Round())
	if err != nil {
		return err
	}

	comm, err := r.GetCommitment(ctx, id)
	if err != nil {
		return err
	}

	signatureData, err := gsp.Signature().Signature()
	if err != nil {
		return err
	}

	signerData, err := gsp.Signature().Signer()
	if err != nil {
		return err
	}

	signature := rebro.Signature{
		Body:   signatureData,
		Signer: signerData,
	}

	if err := bro.signer.Verify(canonicalID, signature); err != nil {
		return err
	}

	_, err = comm.AddSignature(signature)
	if err != nil {
		return err
	}

	return nil
}
