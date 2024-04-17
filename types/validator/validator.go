package validator

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/1ykyk/rebro/types/keys"
)

// MaxStake - the maximum allowed total voting power.
const MaxStake = int64(math.MaxInt64) / 8

type Includer struct {
	PubKey keys.PubKey

	Stake    int64
	priority int64
}

func NewValidator(pK keys.PubKey, Stake int64) *Includer {
	return &Includer{
		PubKey: pK,
		Stake:  Stake,
	}
}

// Validate performs basic validation.
func (i *Includer) Validate() error {
	if i == nil {
		return errors.New("nil includer")
	}
	if i.PubKey == nil {
		return errors.New("validator does not have a public key")
	}

	if i.Stake < 0 {
		return errors.New("validator has negative voting power")
	}
	return nil
}

// Includers contains all available includers (+ the signer),
// sorted by the voting power in a decreasing order
type Includers struct {
	includers []*Includer

	totalStake int64
}

func NewValidatorSet(v []*Includer) *Includers {
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
			return fmt.Errorf("invalid validator #%d: %w", idx, err)
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
				"Total voting power exceeds MaxStake: %v; got: %v",
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
