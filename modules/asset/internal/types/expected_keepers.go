package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Bankx Keeper will implement the interface
type ExpectedBankxKeeper interface {
	DeductFee(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) sdk.Error
	AddCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) sdk.Error
	SubtractCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) sdk.Error

	GetTotalCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
}