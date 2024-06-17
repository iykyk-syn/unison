package quorum

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/iykyk-syn/unison/crypto"
)

// MaxStake - the maximum allowed stake.
const MaxStake = int64(math.MaxInt64) / 8

type Includer struct {
	PubKey crypto.PubKey

	Stake int64
}

func NewIncluder(pK crypto.PubKey, stake int64) *Includer {
	return &Includer{
		PubKey: pK,
		Stake:  stake,
	}
}

// Validate performs basic validation.
func (i *Includer) Validate() error {
	if i == nil {
		return errors.New("nil includer")
	}
	if i.PubKey == nil {
		return errors.New("includer does not have a public key")
	}

	if i.Stake < 0 {
		return errors.New("includer has negative stake")
	}
	return nil
}

// Includers contains all available includers (+ the signer),
// sorted by the stake amount in a decreasing order
type Includers struct {
	includers []*Includer

	totalStake int64
}

func NewIncludersSet(v []*Includer) *Includers {
	set := &Includers{includers: v}
	sort.Sort(set)
	return set
}

func (incl *Includers) Validate() error {
	if incl == nil || len(incl.includers) == 0 {
		return errors.New("includers are nil or empty")
	}

	for idx, i := range incl.includers {
		if err := i.Validate(); err != nil {
			return fmt.Errorf("invalid includer #%d: %w", idx, err)
		}
	}

	return nil
}

func (incl *Includers) GetByPubKey(pubK []byte) *Includer {
	for _, v := range incl.includers {
		if v.PubKey.Equals(pubK) {
			return v
		}
	}
	return nil
}

func (incl *Includers) TotalStake() int64 {
	if incl.totalStake == 0 {
		incl.updateTotalStake()
	}
	return incl.totalStake
}

func (incl *Includers) updateTotalStake() {
	sum := int64(0)
	for _, val := range incl.includers {
		// mind overflow
		sum = safeAddClip(sum, val.Stake)
		if sum > MaxStake {
			panic(fmt.Sprintf(
				"Total stake exceeds MaxStake: %v; got: %v",
				MaxStake,
				sum))
		}
	}
	incl.totalStake = sum
}

func (incl *Includers) Len() int { return len(incl.includers) }

func (incl *Includers) Less(i, j int) bool {
	if incl.includers[i].Stake == incl.includers[j].Stake {
		return bytes.Compare(incl.includers[i].PubKey.Bytes(), incl.includers[j].PubKey.Bytes()) == -1
	}
	return incl.includers[i].Stake > incl.includers[j].Stake
}

func (incl *Includers) Swap(i, j int) {
	incl.includers[i], incl.includers[j] = incl.includers[j], incl.includers[i]
}

func safeAddClip(a, b int64) int64 {
	c, overflow := safeAdd(a, b)
	if overflow {
		if b < 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

func safeAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return -1, true
	} else if b < 0 && a < math.MinInt64-b {
		return -1, true
	}
	return a + b, false
}
