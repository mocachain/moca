package encoding

import (
	"fmt"

	"cosmossdk.io/x/tx/signing"

	amino "github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/eip712"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"
	protov2 "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	enccodec "github.com/mocachain/moca/v2/encoding/codec"
	evmtypes "github.com/mocachain/moca/v2/x/evm/types"
)

// encodingConfig creates a new EncodingConfig and returns it
func MakeConfig() sdktestutil.TestEncodingConfig {
	cdc := amino.NewLegacyAmino()
	signingOptions := signing.Options{
		CustomGetSigners: map[protoreflect.FullName]signing.GetSignersFunc{
			evmtypes.MsgEthereumTxCustomGetSigner.MsgType: evmtypes.MsgEthereumTxCustomGetSigner.Fn,
			protoreflect.FullName("moca.payment.MsgCreatePaymentAccount"): func(msg protov2.Message) ([][]byte, error) {
				creatorField := msg.ProtoReflect().Descriptor().Fields().ByName("creator")
				if creatorField == nil {
					return nil, fmt.Errorf(
						"creator field not found in %s",
						msg.ProtoReflect().Descriptor().FullName(),
					)
				}
				signer, err := sdk.AccAddressFromHexUnsafe(msg.ProtoReflect().Get(creatorField).String())
				if err != nil {
					return nil, err
				}
				return [][]byte{signer}, nil
			},
		},
	}

	interfaceRegistry, _ := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: signingOptions,
	})
	codec := amino.NewProtoCodec(interfaceRegistry)
	enccodec.RegisterLegacyAminoCodec(cdc)
	enccodec.RegisterInterfaces(interfaceRegistry)

	// This is needed for the EIP712 txs because currently is using
	// the deprecated method legacytx.StdSignBytes
	legacytx.RegressionTestingAminoCodec = cdc
	// eip712.SetEncodingConfig(cdc, interfaceRegistry)
	eip712.AminoCodec = cdc
	eip712.ProtoCodec = codec

	return sdktestutil.TestEncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             codec,
		TxConfig:          tx.NewTxConfig(codec, tx.DefaultSignModes),
		Amino:             cdc,
	}
}
