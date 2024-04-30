package quorum

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

var (
	faultDenominator int64 = 3
	faultNumerator   int64 = 2
)

type Quorum struct {
	includers *Includers

	certificates map[string]*certificate
	activeStake  int64
}

func NewQuorum(includers *Includers) *Quorum {
	return &Quorum{
		includers:    includers,
		certificates: make(map[string]*certificate, includers.Len()),
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
		if comm.completed {
			comms = append(comms, comm)
		}
	}
	return comms
}

func (q *Quorum) Finalize() (bool, error) {
	finalized := q.activeStake >= q.stakeRequired()
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
	cert.activeStake = safeAddClip(cert.activeStake, includer.Stake)
	if cert.activeStake > MaxStake {
		panic(fmt.Sprintf(
			"Total stake exceeds MaxStake: %v; got: %v",
			MaxStake,
			q.activeStake))
	}

	completed := cert.activeStake >= q.stakeRequired()
	if completed {
		q.activeStake = safeAddClip(q.activeStake, includer.Stake)
		if q.activeStake > MaxStake {
			panic(fmt.Sprintf(
				"Total stake exceeds MaxStake: %v; got: %v",
				MaxStake,
				q.activeStake))
		}
		cert.completed = true
	}
	return completed, nil
}

func (q *Quorum) stakeRequired() int64 {
	return q.includers.TotalStake()*faultNumerator/faultDenominator + 1
}
