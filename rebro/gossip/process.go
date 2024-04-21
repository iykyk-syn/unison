package gossip

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/iykyk-syn/unison/rebro"
	"github.com/iykyk-syn/unison/rebro/gossip/gossipmsg"
)

func (bro *Broadcaster) processGossip(ctx context.Context, gsp gossipmsg.Gossip) error {
	// TODO: DOS protection idea:
	//  * Ensure only N processGossip routines can exist
	//  * Cancel the oldest routine, if a new one does not fit
	//  * Ensure there is a timeout which routine

	switch gsp.Which() {
	case gossipmsg.Gossip_Which_data:
		bro.log.DebugContext(ctx, "processing data message")
		return bro.processData(ctx, gsp)
	case gossipmsg.Gossip_Which_signature:
		bro.log.DebugContext(ctx, "processing signature message")
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

	id, err := bro.decoder(canonicalID)
	if err != nil {
		return fmt.Errorf("unmarhalling MessageID: %w", err)
	}

	msg := rebro.Message{
		ID:   id,
		Data: data,
	}
	hash, err := bro.hasher.Hash(msg)
	if err != nil {
		return fmt.Errorf("hashing Message for MessageID(%s): %w", id.String(), err)
	}

	if !bytes.Equal(hash, msg.ID.Hash()) {
		return fmt.Errorf("computed Message hash inconsistent with MessageID(%s)", id.String())
	}

	r, err := bro.rounds.GetRound(ctx, id.Round())
	if err != nil {
		return fmt.Errorf("getting round(%d): %w", id.Round(), err)
	}
	// add to quorum and prepare the commitment
	err = r.AddCommitment(ctx, msg)
	if err != nil {
		return fmt.Errorf("adding commitment(%s) to the round(%d): %w", id.String(), id.Round(), err)
	}

	if err = bro.verifier.Verify(ctx, msg); err != nil {
		err = fmt.Errorf("verifying commitment(%s) for round(%d): %w", id.String(), id.Round(), err)
		// it means something is wrong with the message and thus its commitment,
		// so delete it
		deleteErr := r.DeleteCommitment(ctx, id)
		if err != nil {
			err = errors.Join(err, fmt.Errorf("deleting invalid commitment(%s) from round(%d): %w", id.String(), id.Round(), deleteErr))
		}
		return err
	}

	signature, err := bro.signer.Sign(canonicalID)
	if err != nil {
		return fmt.Errorf("signing MessageID(%s) for round(%d): %w", id.String(), id.Round(), err)
	}

	// TODO: Investigate reuse of the message instead of making a new one
	// TODO: Investigate consequences of blocking here on local validation.
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
		return fmt.Errorf("broadcasting signature over MessageID(%s) for round(%d): %w", id.String(), id.Round(), err)
	}

	return nil
}

func (bro *Broadcaster) processSignature(ctx context.Context, gsp gossipmsg.Gossip) error {
	canonicalID, err := gsp.Id()
	if err != nil {
		return err
	}

	id, err := bro.decoder(canonicalID)
	if err != nil {
		return fmt.Errorf("unmarhalling MessageID: %w", err)
	}

	r, err := bro.rounds.GetRound(ctx, id.Round())
	if err != nil {
		return fmt.Errorf("getting round(%d): %w", id.Round(), err)
	}

	// ensure we have the commitment before doing expensive verification
	_, err = r.GetCommitment(ctx, id)
	if err != nil {
		return fmt.Errorf("getting commitment(%s) for the round(%d): %w", id.String(), id.Round(), err)
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
		return fmt.Errorf("verifying signature from(%X) for round(%d): %w", signature.Signer, id.Round(), err)
	}

	err = r.AddSignature(ctx, id, signature)
	if err != nil {
		return fmt.Errorf("adding signature from(%X) to commitment(%s), for round(%d): %w", signature.Signer, id.String(), id.Round(), err)
	}

	return nil
}
