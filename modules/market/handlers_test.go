package market

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/supply"

	"github.com/coinexchain/dex/modules/asset"
	"github.com/coinexchain/dex/modules/authx"
	"github.com/coinexchain/dex/modules/bankx"
	"github.com/coinexchain/dex/modules/market/internal/keepers"
	"github.com/coinexchain/dex/modules/market/internal/types"
	"github.com/coinexchain/dex/modules/msgqueue"
	dex "github.com/coinexchain/dex/types"
)

type testInput struct {
	ctx     sdk.Context
	mk      keepers.Keeper
	handler sdk.Handler
	akp     auth.AccountKeeper
	keys    storeKeys
	cdc     *codec.Codec // mk.cdc
}

func (t testInput) getCoinFromAddr(addr sdk.AccAddress, denom string) (cetCoin sdk.Coin) {
	coins := t.akp.GetAccount(t.ctx, addr).GetCoins()
	for _, coin := range coins {
		if coin.Denom == denom {
			cetCoin = coin
			return
		}
	}
	return
}

func (t testInput) hasCoins(addr sdk.AccAddress, coins sdk.Coins) bool {

	coinsStore := t.akp.GetAccount(t.ctx, addr).GetCoins()
	if len(coinsStore) < len(coins) {
		return false
	}

	for _, coin := range coins {
		find := false
		for _, coinC := range coinsStore {
			if coinC.Denom == coin.Denom {
				find = true
				if coinC.IsEqual(coin) {
					break
				} else {
					return false
				}
			}
		}
		if !find {
			return false
		}
	}

	return true
}

var (
	haveCetAddress            = getAddr("000001")
	notHaveCetAddress         = getAddr("000002")
	forbidAddr                = getAddr("000003")
	stock                     = "tusdt"
	money                     = "teos"
	OriginHaveCetAmount int64 = 1E13
	issueAmount         int64 = 210000000000
)

type storeKeys struct {
	assetCapKey *sdk.KVStoreKey
	authCapKey  *sdk.KVStoreKey
	authxCapKey *sdk.KVStoreKey
	keyParams   *sdk.KVStoreKey
	tkeyParams  *sdk.TransientStoreKey
	marketKey   *sdk.KVStoreKey
	authxKey    *sdk.KVStoreKey
	keyStaking  *sdk.KVStoreKey
	tkeyStaking *sdk.TransientStoreKey
}

func prepareAssetKeeper(t *testing.T, keys storeKeys, cdc *codec.Codec, ctx sdk.Context, addrForbid, tokenForbid bool) types.ExpectedAssetStatusKeeper {
	asset.RegisterCodec(cdc)
	auth.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	supply.RegisterCodec(cdc)

	//create auth, asset keeper
	ak := auth.NewAccountKeeper(
		cdc,
		keys.authCapKey,
		params.NewKeeper(cdc, keys.keyParams, keys.tkeyParams, params.DefaultCodespace).Subspace(auth.DefaultParamspace), auth.ProtoBaseAccount,
	)
	bk := bank.NewBaseKeeper(
		ak,
		params.NewKeeper(cdc, keys.keyParams, keys.tkeyParams, params.DefaultCodespace).Subspace(bank.DefaultParamspace),
		sdk.CodespaceRoot,
	)

	// account permissions
	maccPerms := map[string][]string{
		auth.FeeCollectorName:     {supply.Basic},
		authx.ModuleName:          {supply.Basic},
		distr.ModuleName:          {supply.Basic},
		staking.BondedPoolName:    {supply.Burner, supply.Staking},
		staking.NotBondedPoolName: {supply.Burner, supply.Staking},
		gov.ModuleName:            {supply.Burner},
		types.ModuleName:          {supply.Basic},
	}
	sk := supply.NewKeeper(cdc, keys.keyParams, ak, bk, supply.DefaultCodespace, maccPerms)
	ak.SetAccount(ctx, supply.NewEmptyModuleAccount(authx.ModuleName))

	axk := authx.NewKeeper(
		cdc,
		keys.authxCapKey,
		params.NewKeeper(cdc, keys.keyParams, keys.tkeyParams, params.DefaultCodespace).Subspace(authx.DefaultParamspace),
		sk,
		ak,
	)

	ask := asset.NewBaseTokenKeeper(
		cdc,
		keys.assetCapKey,
	)
	bkx := bankx.NewKeeper(
		params.NewKeeper(cdc, keys.keyParams, keys.tkeyParams, params.DefaultCodespace).Subspace(bankx.DefaultParamspace),
		axk, bk, ak, ask,
		sk,
		msgqueue.NewProducer(),
	)
	tk := asset.NewBaseKeeper(
		cdc,
		keys.assetCapKey,
		params.NewKeeper(cdc, keys.keyParams, keys.tkeyParams, params.DefaultCodespace).Subspace(asset.DefaultParamspace),
		bkx,
		sk,
	)
	tk.SetParams(ctx, asset.DefaultParams())

	// create an account by auth keeper
	cetacc := ak.NewAccountWithAddress(ctx, haveCetAddress)
	coins := dex.NewCetCoins(OriginHaveCetAmount).
		Add(sdk.NewCoins(sdk.NewCoin(stock, sdk.NewInt(issueAmount))))
	cetacc.SetCoins(coins)
	ak.SetAccount(ctx, cetacc)
	usdtacc := ak.NewAccountWithAddress(ctx, forbidAddr)
	usdtacc.SetCoins(sdk.NewCoins(sdk.NewCoin(stock, sdk.NewInt(issueAmount)),
		sdk.NewCoin(dex.CET, sdk.NewInt(issueAmount))))
	ak.SetAccount(ctx, usdtacc)
	onlyIssueToken := ak.NewAccountWithAddress(ctx, notHaveCetAddress)
	onlyIssueToken.SetCoins(dex.NewCetCoins(asset.IssueTokenFee))
	ak.SetAccount(ctx, onlyIssueToken)

	// issue tokens
	msgStock := asset.NewMsgIssueToken(stock, stock, issueAmount, haveCetAddress,
		false, false, addrForbid, tokenForbid, "", "")
	msgMoney := asset.NewMsgIssueToken(money, money, issueAmount, notHaveCetAddress,
		false, false, addrForbid, tokenForbid, "", "")
	msgCet := asset.NewMsgIssueToken("cet", "cet", issueAmount, haveCetAddress,
		false, false, addrForbid, tokenForbid, "", "")
	handler := asset.NewHandler(tk)
	ret := handler(ctx, msgStock)
	require.Equal(t, true, ret.IsOK(), "issue token should succeed", ret)
	ret = handler(ctx, msgMoney)
	require.Equal(t, true, ret.IsOK(), "issue token should succeed", ret)
	ret = handler(ctx, msgCet)
	require.Equal(t, true, ret.IsOK(), "issue token should succeed", ret)

	if tokenForbid {
		msgForbidToken := asset.MsgForbidToken{
			Symbol:       stock,
			OwnerAddress: haveCetAddress,
		}
		tk.ForbidToken(ctx, msgForbidToken.Symbol, msgForbidToken.OwnerAddress)
		msgForbidToken.Symbol = money
		tk.ForbidToken(ctx, msgForbidToken.Symbol, msgForbidToken.OwnerAddress)
	}
	if addrForbid {
		msgForbidAddr := asset.MsgForbidAddr{
			Symbol:    money,
			OwnerAddr: haveCetAddress,
			Addresses: []sdk.AccAddress{forbidAddr},
		}
		tk.ForbidAddress(ctx, msgForbidAddr.Symbol, msgForbidAddr.OwnerAddr, msgForbidAddr.Addresses)
		msgForbidAddr.Symbol = stock
		tk.ForbidAddress(ctx, msgForbidAddr.Symbol, msgForbidAddr.OwnerAddr, msgForbidAddr.Addresses)
	}

	return tk
}

func prepareBankxKeeper(keys storeKeys, cdc *codec.Codec, ctx sdk.Context) types.ExpectedBankxKeeper {
	paramsKeeper := params.NewKeeper(cdc, keys.keyParams, keys.tkeyParams, params.DefaultCodespace)
	producer := msgqueue.NewProducer()
	ak := auth.NewAccountKeeper(cdc, keys.authCapKey, paramsKeeper.Subspace(auth.StoreKey), auth.ProtoBaseAccount)

	bk := bank.NewBaseKeeper(ak, paramsKeeper.Subspace(bank.DefaultParamspace), sdk.CodespaceRoot)
	maccPerms := map[string][]string{
		auth.FeeCollectorName:     {supply.Basic},
		authx.ModuleName:          {supply.Basic},
		distr.ModuleName:          {supply.Basic},
		staking.BondedPoolName:    {supply.Burner, supply.Staking},
		staking.NotBondedPoolName: {supply.Burner, supply.Staking},
		gov.ModuleName:            {supply.Burner},
		types.ModuleName:          {supply.Basic},
	}
	sk := supply.NewKeeper(cdc, keys.keyParams, ak, bk, supply.DefaultCodespace, maccPerms)
	ak.SetAccount(ctx, supply.NewEmptyModuleAccount(authx.ModuleName))

	axk := authx.NewKeeper(cdc, keys.authxKey, paramsKeeper.Subspace(authx.DefaultParamspace), sk, ak)
	ask := asset.NewBaseTokenKeeper(cdc, keys.assetCapKey)
	bxkKeeper := bankx.NewKeeper(paramsKeeper.Subspace("bankx"), axk, bk, ak, ask, sk, producer)
	bk.SetSendEnabled(ctx, true)
	bxkKeeper.SetParam(ctx, bankx.DefaultParams())

	return bxkKeeper
}

func prepareMockInput(t *testing.T, addrForbid, tokenForbid bool) testInput {
	cdc := codec.New()
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)

	keys := storeKeys{}
	keys.marketKey = sdk.NewKVStoreKey(types.StoreKey)
	keys.assetCapKey = sdk.NewKVStoreKey(asset.StoreKey)
	keys.authCapKey = sdk.NewKVStoreKey(auth.StoreKey)
	keys.authxCapKey = sdk.NewKVStoreKey(authx.StoreKey)
	keys.keyParams = sdk.NewKVStoreKey(params.StoreKey)
	keys.tkeyParams = sdk.NewTransientStoreKey(params.TStoreKey)
	keys.authxKey = sdk.NewKVStoreKey(authx.StoreKey)
	keys.keyStaking = sdk.NewKVStoreKey(staking.StoreKey)
	keys.tkeyStaking = sdk.NewTransientStoreKey(staking.TStoreKey)

	ms.MountStoreWithDB(keys.assetCapKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keys.authCapKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keys.keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keys.tkeyParams, sdk.StoreTypeTransient, db)
	ms.MountStoreWithDB(keys.marketKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keys.authxKey, sdk.StoreTypeIAVL, db)
	ms.LoadLatestVersion()

	ctx := sdk.NewContext(ms, abci.Header{ChainID: "test-chain-id"}, false, log.NewNopLogger())
	ak := prepareAssetKeeper(t, keys, cdc, ctx, addrForbid, tokenForbid)
	bk := prepareBankxKeeper(keys, cdc, ctx)

	paramsKeeper := params.NewKeeper(cdc, keys.keyParams, keys.tkeyParams, params.DefaultCodespace)
	mk := keepers.NewKeeper(keys.marketKey, ak, bk, cdc,
		msgqueue.NewProducer(), paramsKeeper.Subspace(types.StoreKey))
	types.RegisterCodec(cdc)

	akp := auth.NewAccountKeeper(cdc, keys.authCapKey, paramsKeeper.Subspace(auth.StoreKey), auth.ProtoBaseAccount)
	// subspace := paramsKeeper.Subspace(StoreKey)
	// keeper := NewKeeper(keys.marketKey, ak, bk, mockFeeKeeper{}, msgCdc, msgqueue.NewProducer(), subspace)
	parameters := keepers.DefaultParams()
	mk.SetParams(ctx, parameters)

	return testInput{ctx: ctx, mk: mk, handler: NewHandler(mk), akp: akp, keys: keys, cdc: cdc}
}

func TestMarketInfoSetFailed(t *testing.T) {
	input := prepareMockInput(t, false, true)
	remainCoin := dex.NewCetCoin(OriginHaveCetAmount + issueAmount - asset.IssueTokenFee*2)
	msgMarket := types.MsgCreateTradingPair{
		Stock:          stock,
		Money:          money,
		Creator:        haveCetAddress,
		PricePrecision: 8,
	}

	// failed by token not exist
	failedToken := msgMarket
	failedToken.Money = "tbtc"
	ret := input.handler(input.ctx, failedToken)
	require.Equal(t, types.CodeInvalidToken, ret.Code, "create market info should failed by token not exist")
	require.Equal(t, true, input.hasCoins(haveCetAddress, sdk.Coins{remainCoin}), "The amount is error")

	failedToken.Stock = "tiota"
	failedToken.Money = money
	ret = input.handler(input.ctx, failedToken)
	require.Equal(t, types.CodeInvalidToken, ret.Code, "create market info should failed by token not exist")
	require.Equal(t, true, input.hasCoins(haveCetAddress, sdk.Coins{remainCoin}), "The amount is error")

	// failed by not token issuer
	failedTokenIssuer := msgMarket
	addr, _ := simpleAddr("00008")
	failedTokenIssuer.Creator = addr
	ret = input.handler(input.ctx, failedTokenIssuer)
	require.Equal(t, types.CodeInvalidTokenIssuer, ret.Code, "create market info should failed by not token issuer")
	require.Equal(t, true, input.hasCoins(haveCetAddress, sdk.Coins{remainCoin}), "The amount is error")

	// failed by price precision
	failedPricePrecision := msgMarket
	failedPricePrecision.Money = "cet"
	failedPricePrecision.PricePrecision = 6
	ret = input.handler(input.ctx, failedPricePrecision)
	require.Equal(t, types.CodeInvalidPricePrecision, ret.Code, "create market info should failed")
	require.Equal(t, true, input.hasCoins(haveCetAddress, sdk.Coins{remainCoin}), "The amount is error")

	failedPricePrecision.PricePrecision = 19
	ret = input.handler(input.ctx, failedPricePrecision)
	require.Equal(t, types.CodeInvalidPricePrecision, ret.Code, "create market info should failed")
	require.Equal(t, true, input.hasCoins(haveCetAddress, sdk.Coins{remainCoin}), "The amount is error")

	// failed by not have sufficient cet
	failedInsufficient := msgMarket
	failedInsufficient.Creator = notHaveCetAddress
	failedInsufficient.Money = "cet"
	failedInsufficient.Stock = money
	ret = input.handler(input.ctx, failedInsufficient)
	require.Equal(t, types.CodeInsufficientCoin, ret.Code, "create market info should failed")
	require.Equal(t, true, input.hasCoins(haveCetAddress, sdk.Coins{remainCoin}), "The amount is error")

	// failed by not have cet trade
	failedNotHaveCetTrade := msgMarket
	ret = input.handler(input.ctx, failedNotHaveCetTrade)
	require.Equal(t, types.CodeStockNoHaveCetTrade, ret.Code, "create market info should failed")
	require.Equal(t, true, input.hasCoins(haveCetAddress, sdk.Coins{remainCoin}), "The amount is error")

}

func createMarket(input testInput) sdk.Result {
	return createImpMarket(input, stock, money)
}

func createImpMarket(input testInput, stock, money string) sdk.Result {
	msgMarketInfo := types.MsgCreateTradingPair{Stock: stock, Money: money, Creator: haveCetAddress, PricePrecision: 8}
	return input.handler(input.ctx, msgMarketInfo)
}

func createCetMarket(input testInput, stock string) sdk.Result {
	return createImpMarket(input, stock, dex.CET)
}

func IsEqual(old, new sdk.Coin, diff sdk.Coin) bool {

	return old.IsEqual(new.Add(diff))
}

func TestMarketInfoSetSuccess(t *testing.T) {
	input := prepareMockInput(t, true, true)
	oldCetCoin := input.getCoinFromAddr(haveCetAddress, dex.CET)
	params := input.mk.GetParams(input.ctx)

	ret := createCetMarket(input, stock)
	newCetCoin := input.getCoinFromAddr(haveCetAddress, dex.CET)
	require.Equal(t, true, ret.IsOK(), "create market info should succeed")
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, dex.NewCetCoin(params.CreateMarketFee)), "The amount is error")

}

func TestCreateOrderFailed(t *testing.T) {
	input := prepareMockInput(t, false, true)
	msgOrder := types.MsgCreateOrder{
		Sender:         haveCetAddress,
		Sequence:       1,
		TradingPair:    stock + types.SymbolSeparator + money,
		OrderType:      types.LimitOrder,
		PricePrecision: 8,
		Price:          100,
		Quantity:       10000000,
		Side:           types.SELL,
		TimeInForce:    types.GTE,
	}
	ret := createCetMarket(input, stock)
	require.Equal(t, true, ret.IsOK(), "create market trade should success")
	ret = createMarket(input)
	require.Equal(t, true, ret.IsOK(), "create market trade should success")
	zeroCet := sdk.NewCoin("cet", sdk.NewInt(0))

	failedSymbolOrder := msgOrder
	failedSymbolOrder.TradingPair = stock + types.SymbolSeparator + "no exsit"
	oldCetCoin := input.getCoinFromAddr(haveCetAddress, dex.CET)
	ret = input.handler(input.ctx, failedSymbolOrder)
	newCetCoin := input.getCoinFromAddr(haveCetAddress, dex.CET)
	require.Equal(t, types.CodeInvalidSymbol, ret.Code, "create GTE order should failed by invalid symbol")
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, zeroCet), "The amount is error")

	failedPricePrecisionOrder := msgOrder
	failedPricePrecisionOrder.PricePrecision = 9
	ret = input.handler(input.ctx, failedPricePrecisionOrder)
	oldCetCoin = input.getCoinFromAddr(haveCetAddress, dex.CET)
	require.Equal(t, types.CodeInvalidPricePrecision, ret.Code, "create GTE order should failed by invalid price precision")
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, zeroCet), "The amount is error")

	failedInsufficientCoinOrder := msgOrder
	failedInsufficientCoinOrder.Quantity = issueAmount * 10
	ret = input.handler(input.ctx, failedInsufficientCoinOrder)
	oldCetCoin = input.getCoinFromAddr(haveCetAddress, dex.CET)
	require.Equal(t, types.CodeInsufficientCoin, ret.Code, "create GTE order should failed by insufficient coin")
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, zeroCet), "The amount is error")

	failedTokenForbidOrder := msgOrder
	ret = input.handler(input.ctx, failedTokenForbidOrder)
	oldCetCoin = input.getCoinFromAddr(haveCetAddress, dex.CET)
	require.Equal(t, types.CodeTokenForbidByIssuer, ret.Code, "create GTE order should failed by token forbidden by issuer")
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, zeroCet), "The amount is error")

	input = prepareMockInput(t, true, false)
	ret = createCetMarket(input, stock)
	require.Equal(t, true, ret.IsOK(), "create market failed")
	ret = createMarket(input)
	require.Equal(t, true, ret.IsOK(), "create market failed")

	failedAddrForbidOrder := msgOrder
	failedAddrForbidOrder.Sender = forbidAddr
	newCetCoin = input.getCoinFromAddr(haveCetAddress, dex.CET)
	ret = input.handler(input.ctx, failedAddrForbidOrder)
	oldCetCoin = input.getCoinFromAddr(haveCetAddress, dex.CET)
	require.Equal(t, types.CodeAddressForbidByIssuer, ret.Code, "create GTE order should failed by token forbidden by issuer")
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, zeroCet), "The amount is error")

}

func TestCreateOrderSuccess(t *testing.T) {
	input := prepareMockInput(t, false, false)
	msgGteOrder := types.MsgCreateOrder{
		Sender:         haveCetAddress,
		Sequence:       1,
		TradingPair:    stock + types.SymbolSeparator + "cet",
		OrderType:      types.LimitOrder,
		PricePrecision: 8,
		Price:          100,
		Quantity:       10000000,
		Side:           types.SELL,
		TimeInForce:    types.GTE,
	}

	param := input.mk.GetParams(input.ctx)

	ret := createCetMarket(input, stock)
	require.Equal(t, true, ret.IsOK(), "create market should succeed")

	oldCoin := input.getCoinFromAddr(haveCetAddress, stock)
	ret = input.handler(input.ctx, msgGteOrder)
	newCoin := input.getCoinFromAddr(haveCetAddress, stock)
	frozenMoney := sdk.NewCoin(stock, sdk.NewInt(msgGteOrder.Quantity))
	require.Equal(t, true, ret.IsOK(), "create GTE order should succeed")
	require.Equal(t, true, IsEqual(oldCoin, newCoin, frozenMoney), "The amount is error")

	glk := keepers.NewGlobalOrderKeeper(input.keys.marketKey, input.cdc)
	order := glk.QueryOrder(input.ctx, assemblyOrderID(haveCetAddress, 1, param.ChainIDVersion))
	require.Equal(t, true, isSameOrderAndMsg(order, msgGteOrder), "order should equal msg")

	msgIOCOrder := types.MsgCreateOrder{
		Sender:         haveCetAddress,
		Sequence:       2,
		TradingPair:    stock + types.SymbolSeparator + "cet",
		OrderType:      types.LimitOrder,
		PricePrecision: 8,
		Price:          300,
		Quantity:       68293762,
		Side:           types.BUY,
		TimeInForce:    types.IOC,
	}

	oldCoin = input.getCoinFromAddr(haveCetAddress, dex.CET)
	ret = input.handler(input.ctx, msgIOCOrder)
	newCoin = input.getCoinFromAddr(haveCetAddress, dex.CET)
	frozenMoney = sdk.NewCoin(dex.CET, calculateAmount(msgIOCOrder.Price, msgIOCOrder.Quantity, msgIOCOrder.PricePrecision).RoundInt())
	frozenFee := sdk.NewCoin(dex.CET, sdk.NewInt(param.FixedTradeFee))
	totalFrozen := frozenMoney.Add(frozenFee)
	require.Equal(t, true, ret.IsOK(), "create Ioc order should succeed ; ", ret.Log)
	require.Equal(t, true, IsEqual(oldCoin, newCoin, totalFrozen), "The amount is error")

	order = glk.QueryOrder(input.ctx, assemblyOrderID(haveCetAddress, 2, param.ChainIDVersion))
	require.Equal(t, true, isSameOrderAndMsg(order, msgIOCOrder), "order should equal msg")
}

func assemblyOrderID(addr sdk.AccAddress, seq uint64, chainIDVersion int64) string {
	return fmt.Sprintf("%s-%d-%d", addr, seq, chainIDVersion)
}

func isSameOrderAndMsg(order *types.Order, msg types.MsgCreateOrder) bool {
	p := sdk.NewDec(msg.Price).Quo(sdk.NewDec(int64(math.Pow10(int(msg.PricePrecision)))))
	samePrice := order.Price.Equal(p)
	return bytes.Equal(order.Sender, msg.Sender) && order.Sequence == msg.Sequence &&
		order.TradingPair == msg.TradingPair && order.OrderType == msg.OrderType && samePrice &&
		order.Quantity == msg.Quantity && order.Side == msg.Side && order.TimeInForce == msg.TimeInForce
}

func getAddr(input string) sdk.AccAddress {
	addr, err := sdk.AccAddressFromHex(input)
	if err != nil {
		panic(err)
	}
	return addr
}

func TestCancelOrderFailed(t *testing.T) {
	input := prepareMockInput(t, false, false)
	createCetMarket(input, stock)
	chainIDVersion := input.mk.GetParams(input.ctx).ChainIDVersion
	cancelOrder := types.MsgCancelOrder{
		Sender:  haveCetAddress,
		OrderID: assemblyOrderID(haveCetAddress, 1, chainIDVersion),
	}

	failedOrderNotExist := cancelOrder
	ret := input.handler(input.ctx, failedOrderNotExist)
	require.Equal(t, types.CodeNotFindOrder, ret.Code, "cancel order should failed by not exist ")

	// create order
	msgIOCOrder := types.MsgCreateOrder{
		Sender:         haveCetAddress,
		Sequence:       2,
		TradingPair:    stock + types.SymbolSeparator + "cet",
		OrderType:      types.LimitOrder,
		PricePrecision: 8,
		Price:          300,
		Quantity:       68293762,
		Side:           types.BUY,
		TimeInForce:    types.IOC,
	}
	ret = input.handler(input.ctx, msgIOCOrder)
	require.Equal(t, true, ret.IsOK(), "create Ioc order should succeed ; ", ret.Log)

	failedNotOrderSender := cancelOrder
	failedNotOrderSender.OrderID = assemblyOrderID(notHaveCetAddress, 2, chainIDVersion)
	ret = input.handler(input.ctx, failedNotOrderSender)
	require.Equal(t, types.CodeNotFindOrder, ret.Code, "cancel order should failed by not match order sender")
}

func TestCancelOrderSuccess(t *testing.T) {
	input := prepareMockInput(t, false, false)
	createCetMarket(input, stock)
	chainIDVersion := input.mk.GetParams(input.ctx).ChainIDVersion

	// create order
	msgIOCOrder := types.MsgCreateOrder{
		Sender:         haveCetAddress,
		Sequence:       2,
		TradingPair:    stock + types.SymbolSeparator + "cet",
		OrderType:      types.LimitOrder,
		PricePrecision: 8,
		Price:          300,
		Quantity:       68293762,
		Side:           types.BUY,
		TimeInForce:    types.IOC,
	}
	ret := input.handler(input.ctx, msgIOCOrder)
	require.Equal(t, true, ret.IsOK(), "create Ioc order should succeed ; ", ret.Log)

	cancelOrder := types.MsgCancelOrder{
		Sender:  haveCetAddress,
		OrderID: assemblyOrderID(haveCetAddress, 2, chainIDVersion),
	}
	ret = input.handler(input.ctx, cancelOrder)
	require.Equal(t, true, ret.IsOK(), "cancel order should succeed ; ", ret.Log)

	remainCoin := sdk.NewCoin(money, sdk.NewInt(issueAmount))
	require.Equal(t, true, input.hasCoins(notHaveCetAddress, sdk.Coins{remainCoin}), "The amount is error ")
}

func TestCancelMarketFailed(t *testing.T) {
	input := prepareMockInput(t, false, false)
	createCetMarket(input, stock)

	msgCancelMarket := types.MsgCancelTradingPair{
		Sender:        haveCetAddress,
		TradingPair:   stock + types.SymbolSeparator + "cet",
		EffectiveTime: time.Now().Unix() + keepers.DefaultMarketMinExpiredTime,
	}

	header := abci.Header{Time: time.Now(), Height: 10}
	input.ctx = input.ctx.WithBlockHeader(header)
	failedTime := msgCancelMarket
	failedTime.EffectiveTime = 10
	ret := input.handler(input.ctx, failedTime)
	require.Equal(t, types.CodeInvalidTime, ret.Code, "cancel order should failed by invalid cancel time")

	failedSymbol := msgCancelMarket
	failedSymbol.TradingPair = stock + types.SymbolSeparator + "not exist"
	ret = input.handler(input.ctx, failedSymbol)
	require.Equal(t, types.CodeInvalidSymbol, ret.Code, "cancel order should failed by invalid symbol")

	failedSender := msgCancelMarket
	failedSender.Sender = notHaveCetAddress
	ret = input.handler(input.ctx, failedSender)
	require.Equal(t, types.CodeNotMatchSender, ret.Code, "cancel order should failed by not match sender")
}

func TestCancelMarketSuccess(t *testing.T) {
	input := prepareMockInput(t, false, false)
	createCetMarket(input, stock)

	msgCancelMarket := types.MsgCancelTradingPair{
		Sender:        haveCetAddress,
		TradingPair:   stock + types.SymbolSeparator + "cet",
		EffectiveTime: keepers.DefaultMarketMinExpiredTime + 10,
	}

	ret := input.handler(input.ctx, msgCancelMarket)
	require.Equal(t, true, ret.IsOK(), "cancel market should success")

	dlk := keepers.NewDelistKeeper(input.keys.marketKey)
	delSymbol := dlk.GetDelistSymbolsBeforeTime(input.ctx, keepers.DefaultMarketMinExpiredTime+10+1)[0]
	if delSymbol != stock+types.SymbolSeparator+"cet" {
		t.Error("Not find del market in store")
	}
}

func TestChargeOrderFee(t *testing.T) {
	input := prepareMockInput(t, false, false)
	ret := createCetMarket(input, stock)
	require.Equal(t, true, ret.IsOK(), "create market should success")
	param := input.mk.GetParams(input.ctx)

	msgOrder := types.MsgCreateOrder{
		Sender:         haveCetAddress,
		Sequence:       2,
		TradingPair:    stock + types.SymbolSeparator + dex.CET,
		OrderType:      types.LimitOrder,
		PricePrecision: 8,
		Price:          300,
		Quantity:       100000000000,
		Side:           types.BUY,
		TimeInForce:    types.IOC,
	}

	// charge fix trade fee, because the stock/cet LastExecutedPrice is zero.
	oldCetCoin := input.getCoinFromAddr(msgOrder.Sender, dex.CET)
	ret = input.handler(input.ctx, msgOrder)
	newCetCoin := input.getCoinFromAddr(msgOrder.Sender, dex.CET)
	frozeCoin := dex.NewCetCoin(calculateAmount(msgOrder.Price, msgOrder.Quantity, msgOrder.PricePrecision).RoundInt64())
	frozeFee := dex.NewCetCoin(param.FixedTradeFee)
	totalFreeze := frozeCoin.Add(frozeFee)
	require.Equal(t, true, ret.IsOK(), "create Ioc order should succeed ; ", ret.Log)
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, totalFreeze), "The amount is error ")

	// If stock is cet symbol, Charge a percentage of the transaction fee,
	ret = createImpMarket(input, dex.CET, stock)
	require.Equal(t, true, ret.IsOK(), "create market should success")
	stockIsCetOrder := msgOrder
	stockIsCetOrder.TradingPair = dex.CET + types.SymbolSeparator + stock
	oldCetCoin = input.getCoinFromAddr(msgOrder.Sender, dex.CET)
	ret = input.handler(input.ctx, stockIsCetOrder)
	newCetCoin = input.getCoinFromAddr(msgOrder.Sender, dex.CET)
	rate := sdk.NewDec(param.MarketFeeRate).Quo(sdk.NewDec(int64(math.Pow10(keepers.MarketFeeRatePrecision))))
	frozeFee = dex.NewCetCoin(sdk.NewDec(stockIsCetOrder.Quantity).Mul(rate).RoundInt64())
	require.Equal(t, true, ret.IsOK(), "create Ioc order should succeed ; ", ret.Log)
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, frozeFee), "The amount is error ")

	marketInfo, err := input.mk.GetMarketInfo(input.ctx, msgOrder.TradingPair)
	require.Equal(t, nil, err, "get %s market failed", msgOrder.TradingPair)
	marketInfo.LastExecutedPrice = sdk.NewDec(12)
	err = input.mk.SetMarket(input.ctx, marketInfo)
	require.Equal(t, nil, err, "set %s market failed", msgOrder.TradingPair)

	// Freeze fee at market execution prices
	oldCetCoin = input.getCoinFromAddr(msgOrder.Sender, dex.CET)
	ret = input.handler(input.ctx, msgOrder)
	newCetCoin = input.getCoinFromAddr(msgOrder.Sender, dex.CET)
	frozeFee = dex.NewCetCoin(marketInfo.LastExecutedPrice.MulInt64(msgOrder.Quantity).Mul(rate).RoundInt64())
	totalFreeze = frozeFee.Add(frozeCoin)
	require.Equal(t, true, ret.IsOK(), "create Ioc order should succeed ; ", ret.Log)
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, totalFreeze), "The amount is error ")
}

func TestModifyPricePrecisionFaild(t *testing.T) {
	input := prepareMockInput(t, false, false)
	createCetMarket(input, stock)

	msg := types.MsgModifyPricePrecision{
		Sender:         haveCetAddress,
		TradingPair:    stock + types.SymbolSeparator + dex.CET,
		PricePrecision: 12,
	}

	msgFailedBySender := msg
	msgFailedBySender.Sender = notHaveCetAddress
	ret := input.handler(input.ctx, msgFailedBySender)
	require.Equal(t, types.CodeNotMatchSender, ret.Code, "the tx should failed by dis match sender")

	msgFailedByPricePrecision := msg
	msgFailedByPricePrecision.PricePrecision = 19
	ret = input.handler(input.ctx, msgFailedByPricePrecision)
	require.Equal(t, types.CodeInvalidPricePrecision, ret.Code, "the tx should failed by dis match sender")

	msgFailedByPricePrecision.PricePrecision = 2
	ret = input.handler(input.ctx, msgFailedByPricePrecision)
	require.Equal(t, types.CodeInvalidPricePrecision, ret.Code, "the tx should failed, the price precision can only be increased")

	msgFailedByInvalidSymbol := msg
	msgFailedByInvalidSymbol.TradingPair = stock + types.SymbolSeparator + "not find"
	ret = input.handler(input.ctx, msgFailedByInvalidSymbol)
	require.Equal(t, types.CodeInvalidSymbol, ret.Code, "the tx should failed by dis match sender")
}

func TestModifyPricePrecisionSuccess(t *testing.T) {
	input := prepareMockInput(t, false, false)
	createCetMarket(input, stock)

	msg := types.MsgModifyPricePrecision{
		Sender:         haveCetAddress,
		TradingPair:    stock + types.SymbolSeparator + dex.CET,
		PricePrecision: 12,
	}

	oldCetCoin := input.getCoinFromAddr(haveCetAddress, dex.CET)
	ret := input.handler(input.ctx, msg)
	newCetCoin := input.getCoinFromAddr(haveCetAddress, dex.CET)
	require.Equal(t, true, ret.IsOK(), "the tx should success")
	require.Equal(t, true, IsEqual(oldCetCoin, newCetCoin, sdk.NewCoin(dex.CET, sdk.NewInt(0))), "the amount is error")
}
