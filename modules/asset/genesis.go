package asset

import (
	"errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GenesisState - all asset state that must be provided at genesis
type GenesisState struct {
	Params Params  `json:"params"`
	Tokens []Token `json:"tokens"`
}

// NewGenesisState - Create a new genesis state
func NewGenesisState(params Params, tokens []Token) GenesisState {
	return GenesisState{
		Params: params,
		Tokens: tokens,
	}
}

// DefaultGenesisState - Return a default genesis state
func DefaultGenesisState() GenesisState {
	return NewGenesisState(DefaultParams(), []Token{})
}

// InitGenesis - Init store state from genesis data
func InitGenesis(ctx sdk.Context, tk TokenKeeper, data GenesisState) {
	tk.SetParams(ctx, data.Params)

	for _, token := range data.Tokens {
		tk.SetToken(ctx, token)
	}
}

// ExportGenesis returns a GenesisState for a given context and keeper
func ExportGenesis(ctx sdk.Context, tk TokenKeeper) GenesisState {
	return NewGenesisState(tk.GetParams(ctx), tk.GetAllTokens(ctx))
}

// ValidateGenesis performs basic validation of asset genesis data returning an
// error for any failed validation criteria.
func (data GenesisState) Validate() error {
	err := data.Params.ValidateGenesis()
	if err != nil {
		return err
	}

	tokenSymbols := make(map[string]interface{})

	for _, token := range data.Tokens {
		err = token.IsValid()
		if err != nil {
			return err
		}

		if _, exists := tokenSymbols[token.GetSymbol()]; exists {
			return errors.New("Duplicate token symbol found during asset ValidateGenesis")
		}

		tokenSymbols[token.GetSymbol()] = nil
	}

	return nil
}
