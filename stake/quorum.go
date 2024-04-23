package stake

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

var (
	faultParameter = 1 / 3
	// threshold is a finalization rule for either a single certificate inside the Quorum
	// or the Quorum itself.
	stakeThreshold = 2*faultParameter + 1
)

type Quorum struct {
	includers *Includers

	certificates map[string]rebro.Certificate
	activeStake  int64
}

func NewQuorum(includers *Includers) *Quorum {
	return &Quorum{
		includers:    includers,
		certificates: make(map[string]rebro.Certificate, includers.Len()),
	}
}

func (q *Quorum) Add(msg rebro.Message) error {
	signer := q.includers.GetByPubKey(msg.ID.Signer())
	if signer == nil {
		return errors.New("signer is not a part of the includers set")
	}

	_, ok := q.certificates[msg.ID.String()]
	if ok {
		return errors.New("certificate exists")
	}

	cert, err := q.newCertificate(msg)
	if err != nil {
		return err
	}
	q.certificates[msg.ID.String()] = cert
	return err
}

func (q *Quorum) Get(id rebro.MessageID) (rebro.Certificate, bool) {
	com, ok := q.certificates[id.String()]
	return com, ok
}

func (q *Quorum) Delete(id rebro.MessageID) bool {
	if _, ok := q.certificates[id.String()]; !ok {
		return false
	}
	delete(q.certificates, id.String())
	return true
}

func (q *Quorum) List() []rebro.Certificate {
	comms := make([]rebro.Certificate, 0, len(q.certificates))
	for _, comm := range q.certificates {
		comms = append(comms, comm)
	}
	return comms
}

func (q *Quorum) Finalize() (bool, error) {
	finalized := q.activeStake >= q.includers.TotalStake()*int64(stakeThreshold)
	return finalized, nil
}

func (q *Quorum) newCertificate(msg rebro.Message) (*certificate, error) {
	return &certificate{
		quorum:     q,
		msg:        msg,
		signatures: make([]crypto.Signature, 0, q.includers.Len()),
	}, nil
}

func (q *Quorum) addSignature(s crypto.Signature, cert *certificate) (bool, error) {
	includer := q.includers.GetByPubKey(s.Signer)
	if includer == nil {
		return false, errors.New("the signer is not a part of includers set")
	}

	for _, signature := range cert.signatures {
		if bytes.Equal(signature.Signer, s.Signer) {
			return false, errors.New("duplicate signature from the signer")
		}
	}

	cert.signatures = append(cert.signatures, s)
	cert.activeStake += includer.Stake
	if cert.activeStake > MaxStake {
		panic(fmt.Sprintf(
			"Total stake exceeds MaxStake: %v; got: %v",
			MaxStake,
			q.activeStake))
	}

	completed := cert.activeStake >= q.includers.TotalStake()*int64(stakeThreshold)
	if completed {
		q.activeStake = safeAddClip(q.activeStake, includer.Stake)
		if q.activeStake > MaxStake {
			panic(fmt.Sprintf(
				"Total stake exceeds MaxStake: %v; got: %v",
				MaxStake,
				q.activeStake))
		}
	}
	return completed, nil
}
