package asset

import (
	"reflect"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestTokenKeeper_IssueToken(t *testing.T) {
	input := setupTestInput()

	type args struct {
		ctx sdk.Context
		msg MsgIssueToken
	}
	tests := []struct {
		name string
		args args
		want sdk.Error
	}{
		{
			"base-case",
			args{
				input.ctx,
				NewMsgIssueToken("ABC Token", "abc", 2100, tAccAddr,
					false, false, false, false),
			},
			nil,
		},
		{
			"case-duplicate",
			args{
				input.ctx,
				NewMsgIssueToken("ABC Token", "abc", 2100, tAccAddr,
					false, false, false, false),
			},
			ErrorDuplicateTokenSymbol("token symbol already exists in store"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := input.tk.IssueToken(tt.args.ctx, tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TokenKeeper.IssueToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenKeeper_TokenStore(t *testing.T) {
	input := setupTestInput()

	// set token
	token1, err := NewToken("ABC token", "abc", 2100, tAccAddr,
		false, false, false, false)
	require.NoError(t, err)
	err = input.tk.SetToken(input.ctx, token1)
	require.NoError(t, err)

	token2, err := NewToken("XYZ token", "xyz", 2100, tAccAddr,
		false, false, false, false)
	require.NoError(t, err)
	err = input.tk.SetToken(input.ctx, token2)
	require.NoError(t, err)

	// get all tokens
	tokens := input.tk.GetAllTokens(input.ctx)
	require.Equal(t, 2, len(tokens))
	require.Contains(t, []string{"abc", "xyz"}, tokens[0].GetSymbol())
	require.Contains(t, []string{"abc", "xyz"}, tokens[1].GetSymbol())

	// remove token
	input.tk.RemoveToken(input.ctx, token1)

	// get token
	res := input.tk.GetToken(input.ctx, token1.GetSymbol())
	require.Nil(t, res)

}

func TestTokenKeeper_TransferOwnership(t *testing.T) {
	input := setupTestInput()
	symbol := "abc"
	var addr1, _ = sdk.AccAddressFromBech32("cosmos1n9e8krs6dengw6k8ts0xpntyzd27rhj48ve5gd")

	//case 1: base-case ok
	// set token
	issueMsg := NewMsgIssueToken("ABC token", symbol, 2100, tAccAddr,
		false, false, false, false)
	err := input.tk.IssueToken(input.ctx, issueMsg)
	require.NoError(t, err)

	msg := NewMsgTransferOwnership(symbol, tAccAddr, addr1)
	err = input.tk.TransferOwnership(input.ctx, msg)
	require.NoError(t, err)

	// get token
	token := input.tk.GetToken(input.ctx, symbol)
	require.NotNil(t, token)
	require.Equal(t, addr1.String(), token.GetOwner().String())

	//case2: invalid token
	msg = NewMsgTransferOwnership("xyz", tAccAddr, addr1)
	err = input.tk.TransferOwnership(input.ctx, msg)
	require.Error(t, err)

	//case3: invalid original owner
	msg = NewMsgTransferOwnership(symbol, tAccAddr, addr1)
	err = input.tk.TransferOwnership(input.ctx, msg)
	require.Error(t, err)

	//case4: invalid new owner
	msg = NewMsgTransferOwnership(symbol, addr1, sdk.AccAddress{})
	err = input.tk.TransferOwnership(input.ctx, msg)
	require.Error(t, err)
}