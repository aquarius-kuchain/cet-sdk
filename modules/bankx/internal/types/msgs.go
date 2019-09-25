package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
)

var _ sdk.Msg = MsgSetMemoRequired{}

type MsgSetMemoRequired struct {
	Address  sdk.AccAddress `json:"address"`
	Required bool           `json:"required"`
}

func NewMsgSetTransferMemoRequired(addr sdk.AccAddress, required bool) MsgSetMemoRequired {
	return MsgSetMemoRequired{Address: addr, Required: required}
}

func (msg *MsgSetMemoRequired) SetAccAddress(addr sdk.AccAddress) {
	msg.Address = addr
}

// --------------------------------------------------------
// sdk.Msg Implementation

func (msg MsgSetMemoRequired) Route() string { return RouterKey }

func (msg MsgSetMemoRequired) Type() string { return "set_memo_required" }

func (msg MsgSetMemoRequired) ValidateBasic() sdk.Error {
	if msg.Address.Empty() {
		return sdk.ErrInvalidAddress("missing address")
	}
	return nil
}

func (msg MsgSetMemoRequired) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

func (msg MsgSetMemoRequired) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Address}
}

var _ sdk.Msg = MsgSend{}

type MsgSend struct {
	FromAddress sdk.AccAddress `json:"from_address"`
	ToAddress   sdk.AccAddress `json:"to_address"`
	Amount      sdk.Coins      `json:"amount"`
	UnlockTime  int64          `json:"unlock_time"`
}

func (msg *MsgSend) SetAccAddress(addr sdk.AccAddress) {
	msg.FromAddress = addr
}

func NewMsgSend(fromAddr, toAddr sdk.AccAddress, amount sdk.Coins, unlockTime int64) MsgSend {
	return MsgSend{FromAddress: fromAddr, ToAddress: toAddr, Amount: amount, UnlockTime: unlockTime}
}

func (msg MsgSend) Route() string {
	return RouterKey
}

func (msg MsgSend) Type() string {
	return "send"
}

func (msg MsgSend) ValidateBasic() sdk.Error {
	if msg.FromAddress.Empty() {
		return sdk.ErrInvalidAddress("missing sender address")
	}
	if msg.ToAddress.Empty() {
		return sdk.ErrInvalidAddress("missing recipient address")
	}
	if !msg.Amount.IsValid() {
		return sdk.ErrInvalidCoins("send amount is invalid: " + msg.Amount.String())
	}
	if !msg.Amount.IsAllPositive() {
		return sdk.ErrInsufficientCoins("send amount must be positive")
	}
	if msg.UnlockTime < 0 {
		return ErrUnlockTime("negative unlock time ")
	}

	return nil
}

func (msg MsgSend) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

func (msg MsgSend) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.FromAddress}
}

// MsgMultiSend - high level transaction of the coin module
type MsgMultiSend struct {
	Inputs  []bank.Input  `json:"inputs" yaml:"inputs"`
	Outputs []bank.Output `json:"outputs" yaml:"outputs"`
}

var _ sdk.Msg = MsgMultiSend{}

// NewMsgMultiSend - construct arbitrary multi-in, multi-out send msg.
func NewMsgMultiSend(in []bank.Input, out []bank.Output) MsgMultiSend {
	return MsgMultiSend{Inputs: in, Outputs: out}
}

// Route Implements Msg
func (msg MsgMultiSend) Route() string { return RouterKey }

// Type Implements Msg
func (msg MsgMultiSend) Type() string { return "multisend" }

// ValidateBasic Implements Msg.
func (msg MsgMultiSend) ValidateBasic() sdk.Error {
	// this just makes sure all the inputs and outputs are properly formatted,
	// not that they actually have the money inside
	if len(msg.Inputs) == 0 {
		return ErrNoInputs()
	}
	if len(msg.Outputs) == 0 {
		return ErrNoOutputs()
	}

	return ValidateInputsOutputs(msg.Inputs, msg.Outputs)
}

// GetSignBytes Implements Msg.
func (msg MsgMultiSend) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners Implements Msg.
func (msg MsgMultiSend) GetSigners() []sdk.AccAddress {
	addrs := make([]sdk.AccAddress, len(msg.Inputs))
	for i, in := range msg.Inputs {
		addrs[i] = in.Address
	}
	return addrs
}

// ValidateInputsOutputs validates that each respective input and output is
// valid and that the sum of inputs is equal to the sum of outputs.
func ValidateInputsOutputs(inputs []bank.Input, outputs []bank.Output) sdk.Error {
	var totalIn, totalOut sdk.Coins

	for _, in := range inputs {
		if err := in.ValidateBasic(); err != nil {
			return err.TraceSDK("")
		}
		totalIn = totalIn.Add(in.Coins)
	}

	for _, out := range outputs {
		if err := out.ValidateBasic(); err != nil {
			return err.TraceSDK("")
		}
		totalOut = totalOut.Add(out.Coins)
	}

	// make sure inputs and outputs match
	if !totalIn.IsEqual(totalOut) {
		return ErrInputOutputMismatch("inputs outputs mismatch")
	}

	return nil
}
