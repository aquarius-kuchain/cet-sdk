package asset

import (
	"bytes"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/params"

	"github.com/coinexchain/dex/types"
)

// DefaultParamspace defines the default asset module parameter subspace
const (
	// ModuleName is the name of the module
	ModuleName = "asset"

	// StoreKey is string representation of the store key for asset
	StoreKey = ModuleName

	// RouterKey is the message route for asset
	RouterKey = ModuleName

	// QuerierRoute is the querier route for asset
	QuerierRoute = ModuleName

	DefaultParamspace = ModuleName
	MaxTokenAmount    = 9E18 // 90 billion * 10 ^ 8
	RareSymbolLength  = 2

	IssueTokenFee     = 1E12 // 10000 * 10 ^8
	IssueRareTokenFee = 1E13 // 100000 * 10 ^8
)

// Parameter keys
var (
	KeyIssueTokenFee     = []byte("IssueTokenFee")
	KeyIssueRareTokenFee = []byte("IssueRareTokenFee")
)

var _ params.ParamSet = &Params{}

// Params defines the parameters for the asset module.
type Params struct {
	// FeeParams define the rules according to which fee are charged.
	IssueTokenFee     sdk.Coins `json:"issue_token_fee"`
	IssueRareTokenFee sdk.Coins `json:"issue_rare_token_fee"`
}

// ParamKeyTable for asset module
func ParamKeyTable() params.KeyTable {
	return params.NewKeyTable().RegisterParamSet(&Params{})
}

// ParamSetPairs implements the ParamSet interface and returns all the key/value pairs
// pairs of asset module's parameters.
func (p *Params) ParamSetPairs() params.ParamSetPairs {
	return params.ParamSetPairs{
		{Key: KeyIssueTokenFee, Value: &p.IssueTokenFee},
		{Key: KeyIssueRareTokenFee, Value: &p.IssueRareTokenFee},
	}
}

// Equal returns a boolean determining if two Params types are identical.
func (p Params) Equal(p2 Params) bool {
	bz1 := msgCdc.MustMarshalBinaryLengthPrefixed(&p)
	bz2 := msgCdc.MustMarshalBinaryLengthPrefixed(&p2)
	return bytes.Equal(bz1, bz2)
}

func (p *Params) ValidateGenesis() error {
	for _, pair := range p.ParamSetPairs() {
		fee := pair.Value.(*sdk.Coins)
		if fee.Empty() || fee.IsAnyNegative() {
			return fmt.Errorf("%s must be a valid sdk.Coins, is %s", pair.Key, fee.String())
		}
	}
	return nil
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return Params{
		types.NewCetCoins(IssueTokenFee),
		types.NewCetCoins(IssueRareTokenFee),
	}
}
