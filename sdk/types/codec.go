package types

import (
	"fmt"

	feegranttypes "cosmossdk.io/x/feegrant"
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
	mocatypes "github.com/mocachain/moca/v2/types"
	challengetypes "github.com/mocachain/moca/v2/x/challenge/types"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
	protov2 "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Codec() *codec.ProtoCodec {
	// cosmos-sdk v0.53 requires address codecs on signing.Options; without them
	// the registry has no signing context and downstream tx-config construction
	// (authtx.NewTxConfig*) errors with "address codec is required".
	interfaceRegistry, err := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          cmdcfg.NewMultiPrefixBech32AccCodec(),
			ValidatorAddressCodec: cmdcfg.NewMultiPrefixBech32ValCodec(),
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
		},
	})
	if err != nil {
		panic(err)
	}
	challengetypes.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	mocatypes.RegisterInterfaces(interfaceRegistry)
	authtypes.RegisterInterfaces(interfaceRegistry)
	authztypes.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	distrtypes.RegisterInterfaces(interfaceRegistry)
	feegranttypes.RegisterInterfaces(interfaceRegistry)
	slashingtypes.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	sptypes.RegisterInterfaces(interfaceRegistry)
	paymenttypes.RegisterInterfaces(interfaceRegistry)
	storagetypes.RegisterInterfaces(interfaceRegistry)
	govv1.RegisterInterfaces(interfaceRegistry)
	consensustypes.RegisterInterfaces(interfaceRegistry)
	// evidencetypes.RegisterInterfaces is not available in cosmossdk.io/x/evidence
	// Evidence interfaces are registered via module manager in app.go
	minttypes.RegisterInterfaces(interfaceRegistry)
	vgtypes.RegisterInterfaces(interfaceRegistry)

	return codec.NewProtoCodec(interfaceRegistry)
}
