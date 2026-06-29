package backend

import (
	"fmt"

	sdkmath "cosmossdk.io/math"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	goethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"google.golang.org/grpc/metadata"

	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/rpc/backend/mocks"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
)

func (suite *BackendTestSuite) TestSendTransaction() {
	gasPrice := new(hexutil.Big)
	// 21000 is the intrinsic-gas floor for a plain transfer; cosmos/evm v0.6.0's
	// MsgEthereumTx.ValidateBasic enforces it (the old in-tree x/evm did not), so
	// the happy-path fixture must meet it. The gas-set-to-0 case below still uses
	// zeroGas to assert that floor is enforced.
	gas := hexutil.Uint64(21000)
	zeroGas := hexutil.Uint64(0)
	toAddr := utiltx.GenerateAddress()
	priv, _ := ethsecp256k1.GenPrivKey()
	from := common.BytesToAddress(priv.PubKey().Address().Bytes())
	nonce := hexutil.Uint64(1)
	baseFee := sdkmath.NewInt(1)
	callArgsDefault := evmtypes.TransactionArgs{
		From:     &from,
		To:       &toAddr,
		GasPrice: gasPrice,
		Gas:      &gas,
		Nonce:    &nonce,
	}

	hash := common.Hash{}

	testCases := []struct {
		name         string
		registerMock func()
		args         evmtypes.TransactionArgs
		expHash      common.Hash
		expPass      bool
	}{
		{
			"fail - Can't find account in Keyring",
			func() {},
			evmtypes.TransactionArgs{},
			hash,
			false,
		},
		{
			"fail - Block error can't set Tx defaults",
			func() {
				var header metadata.MD
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				armor := crypto.EncryptArmorPrivKey(priv, "", "eth_secp256k1")
				err := suite.backend.clientCtx.Keyring.ImportPrivKey("test_key", armor, "")
				suite.Require().NoError(err)
				RegisterParams(queryClient, &header, 1)
				RegisterBlockError(client, 1)
			},
			callArgsDefault,
			hash,
			false,
		},
		{
			"fail - Cannot validate transaction gas set to 0",
			func() {
				var header metadata.MD
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				armor := crypto.EncryptArmorPrivKey(priv, "", "eth_secp256k1")
				err := suite.backend.clientCtx.Keyring.ImportPrivKey("test_key", armor, "")
				suite.Require().NoError(err)
				RegisterParams(queryClient, &header, 1)
				_, err = RegisterBlock(client, 1, nil)
				suite.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
				RegisterBaseFee(queryClient, baseFee)
				RegisterParamsWithoutHeader(queryClient, 1)
			},
			evmtypes.TransactionArgs{
				From:     &from,
				To:       &toAddr,
				GasPrice: gasPrice,
				Gas:      &zeroGas,
				Nonce:    &nonce,
			},
			hash,
			false,
		},
		{
			"fail - Cannot broadcast transaction",
			func() {
				client, txBytes := broadcastTx(suite, priv, baseFee, callArgsDefault)
				RegisterBroadcastTxError(client, txBytes)
			},
			callArgsDefault,
			common.Hash{},
			false,
		},
		{
			"pass - Return the transaction hash",
			func() {
				client, txBytes := broadcastTx(suite, priv, baseFee, callArgsDefault)
				RegisterBroadcastTx(client, txBytes)
			},
			callArgsDefault,
			hash,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("case %s", tc.name), func() {
			suite.SetupTest() // reset test and queries
			tc.registerMock()

			if tc.expPass {
				// cosmos/evm v0.6.0: TransactionArgs.ToTransaction returns a
				// *ethtypes.Transaction; sign it with the geth signer and the
				// raw private key, then wrap into a MsgEthereumTx to obtain the
				// expected hash (mirrors rpc/backend/call_tx.go SendRawTransaction).
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParamsWithoutHeader(queryClient, 1)
				ethSigner := ethtypes.LatestSigner(suite.backend.ChainConfig())
				tx := callArgsDefault.ToTransaction(ethtypes.LegacyTxType)
				ecdsaKey, err := priv.ToECDSA()
				suite.Require().NoError(err)
				signedTx, err := ethtypes.SignTx(tx, ethSigner, ecdsaKey)
				suite.Require().NoError(err)
				msg := &evmtypes.MsgEthereumTx{}
				msg.FromEthereumTx(signedTx)
				tc.expHash = msg.AsTransaction().Hash()
			}
			responseHash, err := suite.backend.SendTransaction(tc.args)
			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(tc.expHash, responseHash)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestSign() {
	from, priv := utiltx.NewAddrKey()
	testCases := []struct {
		name         string
		registerMock func()
		fromAddr     common.Address
		inputBz      hexutil.Bytes
		expPass      bool
	}{
		{
			"fail - can't find key in Keyring",
			func() {},
			from,
			nil,
			false,
		},
		{
			"pass - sign nil data",
			func() {
				armor := crypto.EncryptArmorPrivKey(priv, "", "eth_secp256k1")
				err := suite.backend.clientCtx.Keyring.ImportPrivKey("test_key", armor, "")
				suite.Require().NoError(err)
			},
			from,
			nil,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("case %s", tc.name), func() {
			suite.SetupTest() // reset test and queries
			tc.registerMock()

			responseBz, err := suite.backend.Sign(tc.fromAddr, tc.inputBz)
			if tc.expPass {
				signature, _, err := suite.backend.clientCtx.Keyring.SignByAddress((sdk.AccAddress)(from.Bytes()), tc.inputBz, signing.SignMode_SIGN_MODE_DIRECT)
				signature[goethcrypto.RecoveryIDOffset] += 27
				suite.Require().NoError(err)
				suite.Require().Equal((hexutil.Bytes)(signature), responseBz)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestSignTypedData() {
	from, priv := utiltx.NewAddrKey()
	testCases := []struct {
		name           string
		registerMock   func()
		fromAddr       common.Address
		inputTypedData apitypes.TypedData
		expPass        bool
	}{
		{
			"fail - can't find key in Keyring",
			func() {},
			from,
			apitypes.TypedData{},
			false,
		},
		{
			"fail - empty TypeData",
			func() {
				armor := crypto.EncryptArmorPrivKey(priv, "", "eth_secp256k1")
				err := suite.backend.clientCtx.Keyring.ImportPrivKey("test_key", armor, "")
				suite.Require().NoError(err)
			},
			from,
			apitypes.TypedData{},
			false,
		},
		// TODO: Generate a TypedData msg
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("case %s", tc.name), func() {
			suite.SetupTest() // reset test and queries
			tc.registerMock()

			responseBz, err := suite.backend.SignTypedData(tc.fromAddr, tc.inputTypedData)

			if tc.expPass {
				sigHash, _, _ := apitypes.TypedDataAndHash(tc.inputTypedData)
				signature, _, err := suite.backend.clientCtx.Keyring.SignByAddress((sdk.AccAddress)(from.Bytes()), sigHash, signing.SignMode_SIGN_MODE_DIRECT)
				signature[goethcrypto.RecoveryIDOffset] += 27
				suite.Require().NoError(err)
				suite.Require().Equal((hexutil.Bytes)(signature), responseBz)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func broadcastTx(suite *BackendTestSuite, priv *ethsecp256k1.PrivKey, baseFee math.Int, callArgsDefault evmtypes.TransactionArgs) (client *mocks.Client, txBytes []byte) {
	var header metadata.MD
	queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
	client = suite.backend.clientCtx.Client.(*mocks.Client)
	armor := crypto.EncryptArmorPrivKey(priv, "", "eth_secp256k1")
	_ = suite.backend.clientCtx.Keyring.ImportPrivKey("test_key", armor, "")
	RegisterParams(queryClient, &header, 1)
	_, err := RegisterBlock(client, 1, nil)
	suite.Require().NoError(err)
	_, err = RegisterBlockResults(client, 1)
	suite.Require().NoError(err)
	RegisterBaseFee(queryClient, baseFee)
	RegisterParamsWithoutHeader(queryClient, 1)
	ethSigner := ethtypes.LatestSigner(suite.backend.ChainConfig())
	// cosmos/evm v0.6.0: TransactionArgs.ToTransaction returns a geth
	// *ethtypes.Transaction; sign it with the raw private key and wrap into a
	// MsgEthereumTx (mirrors rpc/backend/call_tx.go SendRawTransaction).
	ethTx := callArgsDefault.ToTransaction(ethtypes.LegacyTxType)
	ecdsaKey, err := priv.ToECDSA()
	suite.Require().NoError(err)
	signedTx, err := ethtypes.SignTx(ethTx, ethSigner, ecdsaKey)
	suite.Require().NoError(err)
	msg := &evmtypes.MsgEthereumTx{}
	// Recover and set MsgEthereumTx.From: SendTransaction builds the message via
	// NewTxFromArgs + Sign, which populates From (BuildTx needs the signer), so
	// the expected bytes must include it too.
	suite.Require().NoError(msg.FromSignedEthereumTx(signedTx, ethSigner))
	tx, _ := msg.BuildTx(suite.backend.clientCtx.TxConfig.NewTxBuilder(), evmtypes.DefaultEVMDenom)
	txEncoder := suite.backend.clientCtx.TxConfig.TxEncoder()
	txBytes, _ = txEncoder(tx)
	return client, txBytes
}
