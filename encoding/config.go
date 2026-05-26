package encoding

import (
	"cosmossdk.io/x/tx/signing"
	amino "github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/eip712"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
	enccodec "github.com/mocachain/moca/v2/encoding/codec"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// encodingConfig creates a new EncodingConfig and returns it
func MakeConfig() sdktestutil.TestEncodingConfig {
	cdc := amino.NewLegacyAmino()
	// cosmos-sdk v0.53 requires AddressCodec/ValidatorAddressCodec on
	// signing.Options; without them NewInterfaceRegistryWithOptions errors.
	signingOptions := signing.Options{
		AddressCodec:          cmdcfg.NewMultiPrefixBech32AccCodec(),
		ValidatorAddressCodec: cmdcfg.NewMultiPrefixBech32ValCodec(),
		CustomGetSigners: map[protoreflect.FullName]signing.GetSignersFunc{
			evmtypes.MsgEthereumTxCustomGetSigner.MsgType: evmtypes.MsgEthereumTxCustomGetSigner.Fn,
		},
	}

	interfaceRegistry, err := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: signingOptions,
	})
	if err != nil {
		panic(err)
	}
	codec := amino.NewProtoCodec(interfaceRegistry)
	enccodec.RegisterLegacyAminoCodec(cdc)
	enccodec.RegisterInterfaces(interfaceRegistry)

	// This is needed for the EIP712 txs because currently is using
	// the deprecated method legacytx.StdSignBytes
	legacytx.RegressionTestingAminoCodec = cdc
	// eip712.SetEncodingConfig(cdc, interfaceRegistry)
	eip712.AminoCodec = cdc
	eip712.ProtoCodec = codec

	// cosmos-sdk v0.53: the bare tx.NewTxConfig falls back to empty signing
	// options (no address codec) and panics. Build the tx config with the
	// signing context from the interface registry instead.
	txConfig, err := tx.NewTxConfigWithOptions(codec, tx.ConfigOptions{
		EnabledSignModes: tx.DefaultSignModes,
		SigningContext:   interfaceRegistry.SigningContext(),
	})
	if err != nil {
		panic(err)
	}

	return sdktestutil.TestEncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             codec,
		TxConfig:          txConfig,
		Amino:             cdc,
	}
}
