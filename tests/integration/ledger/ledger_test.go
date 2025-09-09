package ledger_test

import (
	"bytes"
	"context"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/evmos/evmos/v12/encoding"
	"github.com/evmos/evmos/v12/tests/integration/ledger/mocks"
	"github.com/evmos/evmos/v12/testutil"
	utiltx "github.com/evmos/evmos/v12/testutil/tx"

	"github.com/spf13/cobra"

	eip712 "github.com/cosmos/cosmos-sdk/crypto/keys/eth/eip712"
	ethsecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	"github.com/ethereum/go-ethereum/crypto"

	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdktestutilcli "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutilmod "github.com/cosmos/cosmos-sdk/types/module/testutil"
	bankcli "github.com/cosmos/cosmos-sdk/x/bank/client/cli"

	. "github.com/onsi/ginkgo/v2" //nolint
)

var (
	signOkMock = func(_ []uint32, msg []byte) ([]byte, error) {
		// Use the same private key to generate a valid signature
		sig, err := s.privKey.Sign(msg)
		if err != nil {
			return nil, err
		}
		// Return signature in the format expected by Ledger validation
		return sig, nil
	}

	signErrMock = func([]uint32, []byte) ([]byte, error) {
		return nil, mocks.ErrMockedSigning
	}

	// signOkEIP712Mock replicates the real Ledger path for CLI: build EIP-712 digest and sign it
	signOkEIP712Mock = func(_ []uint32, signDocBytes []byte) ([]byte, error) {
		// First, try to follow real Ledger flow: decode SignDoc and build EIP-712 digest
		if typedData, err := eip712.GetEIP712TypedDataForMsg(signDocBytes); err == nil {
			domainSep, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
			if err != nil {
				return nil, err
			}
			msgHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
			if err != nil {
				return nil, err
			}
			digest := crypto.Keccak256([]byte{0x19, 0x01}, domainSep, msgHash)
			// Return 65-byte [R||S||V] with V in {0,1}
			if pk, ok := s.privKey.(*ethsecp256k1.PrivKey); ok {
				ecdsaKey, err := pk.ToECDSA()
				if err != nil {
					return nil, err
				}
				return crypto.Sign(digest, ecdsaKey)
			}
			// Fallback to 64-byte if cast fails
			return s.privKey.Sign(digest)
		}
		// Fallback: some sign-mode(s) may provide raw bytes not decodable as SignDoc
		// In that case, treat as message hash and produce 65-byte signature when possible
		if pk, ok := s.privKey.(*ethsecp256k1.PrivKey); ok {
			ecdsaKey, err := pk.ToECDSA()
			if err != nil {
				return nil, err
			}
			return crypto.Sign(crypto.Keccak256(signDocBytes), ecdsaKey)
		}
		return s.privKey.Sign(crypto.Keccak256(signDocBytes))
	}
)

var _ = Describe("Ledger CLI and keyring functionality: ", func() {
	var (
		receiverAccAddr sdk.AccAddress
		encCfg          sdktestutilmod.TestEncodingConfig
		kr              keyring.Keyring
		mockedIn        sdktestutil.BufferReader
		clientCtx       client.Context
		ctx             context.Context
		cmd             *cobra.Command
		krHome          string
		keyRecord       *keyring.Record
	)

	ledgerKey := "ledger_key"

	s.SetupTest()
	s.SetupEvmosApp()

	Describe("Adding a key from ledger using the CLI", func() {
		BeforeEach(func() {
			krHome = s.T().TempDir()
			encCfg = encoding.MakeConfig()

			cmd = s.evmosAddKeyCmd()

			mockedIn = sdktestutil.ApplyMockIODiscardOutErr(cmd)

			kr, clientCtx, ctx = s.NewKeyringAndCtxs(krHome, mockedIn, encCfg)

			mocks.MClose(s.ledger)
			mocks.MGetAddressPubKeySECP256K1(s.ledger, s.accAddr, s.pubKey)
		})
		Context("with default algo", func() {
			It("should use eth_secp256k1 by default and pass", func() {
				out, err := sdktestutilcli.ExecTestCLICmd(clientCtx, cmd, []string{
					ledgerKey,
					s.FormatFlag(flags.FlagUseLedger),
				})

				s.Require().NoError(err)
				s.Require().Contains(out.String(), "name: ledger_key")

				_, err = kr.Key(ledgerKey)
				s.Require().NoError(err, "can't find ledger key")
			})
		})
		Context("with eth_secp256k1 algo", func() {
			It("should add the ledger key ", func() {
				out, err := sdktestutilcli.ExecTestCLICmd(clientCtx, cmd, []string{
					ledgerKey,
					s.FormatFlag(flags.FlagUseLedger),
					s.FormatFlag(flags.FlagKeyType),
					string(hd.EthSecp256k1Type),
				})

				s.Require().NoError(err)
				s.Require().Contains(out.String(), "name: ledger_key")

				_, err = kr.Key(ledgerKey)
				s.Require().NoError(err, "can't find ledger key")
			})
		})
	})
	Describe("Singing a transactions", func() {
		BeforeEach(func() {
			krHome = s.T().TempDir()
			encCfg = encoding.MakeConfig()

			var err error

			// create add key command
			cmd = s.evmosAddKeyCmd()

			mockedIn = sdktestutil.ApplyMockIODiscardOutErr(cmd)
			mocks.MGetAddressPubKeySECP256K1(s.ledger, s.accAddr, s.pubKey)

			kr, clientCtx, ctx = s.NewKeyringAndCtxs(krHome, mockedIn, encCfg)

			b := bytes.NewBufferString("")
			cmd.SetOut(b)

			cmd.SetArgs([]string{
				ledgerKey,
				s.FormatFlag(flags.FlagUseLedger),
				s.FormatFlag(flags.FlagKeyType),
				"eth_secp256k1",
			})
			// add ledger key for following tests
			s.Require().NoError(cmd.ExecuteContext(ctx))
			keyRecord, err = kr.Key(ledgerKey)
			s.Require().NoError(err, "can't find ledger key")
		})
		Context("perform bank send", func() {
			Context("with keyring functions calling", func() {
				BeforeEach(func() {
					s.ledger = mocks.NewSECP256K1(s.T())

					mocks.MClose(s.ledger)
					mocks.MGetPublicKeySECP256K1(s.ledger, s.pubKey)
				})
				It("should return valid signature", func() {
					mocks.MSignSECP256K1(s.ledger, signOkMock, nil)

					ledgerAddr, err := keyRecord.GetAddress()
					s.Require().NoError(err, "can't retirieve ledger addr from a keyring")

					msg := []byte("test message")

					signed, _, err := kr.SignByAddress(ledgerAddr, msg, signingtypes.SignMode_SIGN_MODE_TEXTUAL)
					s.Require().NoError(err, "failed to sign message")

					valid := s.pubKey.VerifySignature(msg, signed)
					s.Require().True(valid, "invalid signature returned")
				})
				It("should raise error from ledger sign function to the top", func() {
					mocks.MSignSECP256K1(s.ledger, signErrMock, mocks.ErrMockedSigning)

					ledgerAddr, err := keyRecord.GetAddress()
					s.Require().NoError(err, "can't retirieve ledger addr from a keyring")

					msg := []byte("test message")

					_, _, err = kr.SignByAddress(ledgerAddr, msg, signingtypes.SignMode_SIGN_MODE_TEXTUAL)

					s.Require().Error(err, "false positive result, error expected")

					s.Require().Equal(mocks.ErrMockedSigning.Error(), err.Error(), "original and returned errors are not equal")
				})
			})
			Context("with cli command", func() {
				BeforeEach(func() {
					s.ledger = mocks.NewSECP256K1(s.T())

					err := testutil.FundAccount(
						s.ctx,
						s.app.BankKeeper,
						s.accAddr,
						sdk.NewCoins(
							sdk.NewCoin("amoca", sdkmath.NewInt(100000000000000)),
						),
					)
					s.Require().NoError(err)

					receiverAccAddr = sdk.AccAddress(utiltx.GenerateAddress().Bytes())

					cmd = bankcli.NewSendTxCmd()
					mockedIn = sdktestutil.ApplyMockIODiscardOutErr(cmd)

					kr, clientCtx, ctx = s.NewKeyringAndCtxs(krHome, mockedIn, encCfg)

					// register mocked funcs
					mocks.MClose(s.ledger)
					mocks.MGetPublicKeySECP256K1(s.ledger, s.pubKey)
					mocks.MEnsureExist(s.accRetriever, nil)
					mocks.MGetAccountNumberSequence(s.accRetriever, 0, 0, nil)
				})
				It("should execute bank tx cmd", func() {
					mocks.MSignSECP256K1(s.ledger, signOkEIP712Mock, nil)

					cmd.SetContext(ctx)
					cmd.SetArgs([]string{
						ledgerKey,
						receiverAccAddr.String(),
						sdk.NewCoin("amoca", sdkmath.NewInt(1000)).String(),
						s.FormatFlag(flags.FlagUseLedger),
						s.FormatFlag(flags.FlagSkipConfirmation),
						"--sign-mode", "textual",
					})
					out := bytes.NewBufferString("")
					cmd.SetOutput(out)

					err := cmd.Execute()

					s.Require().NoError(err, "can't execute cli tx command")
				})
				It("should return error from ledger device", func() {
					mocks.MSignSECP256K1(s.ledger, signErrMock, mocks.ErrMockedSigning)

					cmd.SetContext(ctx)
					cmd.SetArgs([]string{
						ledgerKey,
						receiverAccAddr.String(),
						sdk.NewCoin("amoca", sdkmath.NewInt(1000)).String(),
						s.FormatFlag(flags.FlagUseLedger),
						s.FormatFlag(flags.FlagSkipConfirmation),
						"--sign-mode", "textual",
					})
					out := bytes.NewBufferString("")
					cmd.SetOutput(out)

					err := cmd.Execute()

					s.Require().Error(err, "false positive, error expected")
					s.Require().Equal(mocks.ErrMockedSigning.Error(), err.Error())
				})
			})
		})
	})
})
