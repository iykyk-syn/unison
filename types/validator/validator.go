package validator

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/1ykyk/rebro/types/keys"
)

// MaxTotalVotingPower - the maximum allowed total voting power.
const MaxTotalVotingPower = int64(math.MaxInt64) / 8

type Validator struct {
	Address keys.Address
	PubKey  keys.PubKey

	VotingPower int64
	priority    int64
}

func NewValidator(pK keys.PubKey, votingPower int64) *Validator {
	return &Validator{
		Address:     pK.Address(),
		PubKey:      pK,
		VotingPower: votingPower,
	}
}

// ValidateBasic performs basic validation.
func (v *Validator) ValidateBasic() error {
	if v == nil {
		return errors.New("nil validator")
	}
	if v.PubKey == nil {
		return errors.New("validator does not have a public key")
	}

	if v.VotingPower < 0 {
		return errors.New("validator has negative voting power")
	}

	if len(v.Address) != keys.AddressSize {
		return fmt.Errorf("validator address is the wrong size: %v", v.Address)
	}
	return nil
}

// ValidatorSet contains all available validators (including the signer),
// sorted by the voting power in a decreasing order
type ValidatorSet struct {
	validators []*Validator

	totalVotingPower int64
}

func NewValidatorSet(v []*Validator) *ValidatorSet {
	set := &ValidatorSet{validators: v}
	sort.Sort(set)
	return set
}

func (vals *ValidatorSet) ValidateBasic() error {
	if vals == nil || len(vals.validators) == 0 {
		return errors.New("validator set is nil or empty")
	}

	for idx, val := range vals.validators {
		if err := val.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid validator #%d: %w", idx, err)
		}
	}

	return nil
}

func (vals *ValidatorSet) GetByPubKey(pubK []byte) *Validator {
	for _, v := range vals.validators {
		if v.PubKey.Equals(pubK) {
			return v
		}
	}
	return nil
}

func (vals *ValidatorSet) TotalVotingPower() int64 {
	if vals.totalVotingPower == 0 {
		vals.updateTotalVotingPower()
	}
	return vals.totalVotingPower
}

func (vals *ValidatorSet) updateTotalVotingPower() {
	sum := int64(0)
	for _, val := range vals.validators {
		// mind overflow
		sum = safeAddClip(sum, val.VotingPower)
		if sum > MaxTotalVotingPower {
			panic(fmt.Sprintf(
				"Total voting power exceeds MaxTotalVotingPower: %v; got: %v",
				MaxTotalVotingPower,
				sum))
		}
	}
	vals.totalVotingPower = sum
}

func (vals *ValidatorSet) Len() int { return len(vals.validators) }

func (vals *ValidatorSet) Less(i, j int) bool {
	if vals.validators[i].VotingPower == vals.validators[j].VotingPower {
		return bytes.Compare(vals.validators[i].Address, vals.validators[j].Address) == -1
	}
	return vals.validators[i].VotingPower > vals.validators[j].VotingPower
}

func (vals *ValidatorSet) Swap(i, j int) {
	vals.validators[i], vals.validators[j] = vals.validators[j], vals.validators[i]
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
