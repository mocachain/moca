package types

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

const (
	// validEndpoint is a well-formed endpoint shared by the ValidateBasic
	// happy-path cases below (keeps the literal out of goconst's way).
	validEndpoint = "http://127.0.0.1:9033"
	// badAddress is a string that is neither a bech32 address nor 40 hex
	// chars, so sdk.AccAddressFromHexUnsafe always rejects it.
	badAddress = "not-a-valid-address"
	// deadbeefHex is a malformed hex payload reused across several
	// wrong-length BLS key/proof/signature test cases.
	deadbeefHex = "deadbeef"
	// badSPAddressCase is the shared test case name for an empty/invalid
	// storage-provider address.
	badSPAddressCase = "bad sp address"
	// validCase is the shared test case name for a well-formed input.
	validCase = "valid"
)

var (
	coinPos  = sdk.NewInt64Coin(DefaultDepositDenom, 100000)
	coinZero = sdk.NewInt64Coin(DefaultDepositDenom, 0)
)

func TestMsgCreateStorageProvider_ValidateBasic(t *testing.T) {
	pk1 := ed25519.GenPrivKey().PubKey()
	spAddr := sdk.AccAddress(pk1.Address())
	blsPubKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	tests := []struct {
		name, moniker, identity, website, details                                                       string
		creator, spAddress, fundingAddress, sealAddress, approvalAddress, gcAddress, maintenanceAddress sdk.AccAddress
		blsKey, blsProof                                                                                string
		deposit                                                                                         sdk.Coin
		err                                                                                             error
	}{
		{"basic", "a", "b", "c", "d", spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, blsPubKey, blsProof, coinPos, nil},
		{"basic_empty", "a", "b", "c", "d", sdk.AccAddress{}, spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, blsPubKey, blsProof, coinPos, sdkerrors.ErrInvalidAddress},
		{"zero deposit", "a", "b", "c", "d", spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, blsPubKey, blsProof, coinZero, sdkerrors.ErrInvalidCoins},
		{"bad operator address", "a", "b", "c", "d", spAddr, sdk.AccAddress{}, spAddr, spAddr, spAddr, spAddr, spAddr, blsPubKey, blsProof, coinPos, sdkerrors.ErrInvalidAddress},
		{"bad funding address", "a", "b", "c", "d", spAddr, spAddr, sdk.AccAddress{}, spAddr, spAddr, spAddr, spAddr, blsPubKey, blsProof, coinPos, sdkerrors.ErrInvalidAddress},
		{"bad seal address", "a", "b", "c", "d", spAddr, spAddr, spAddr, sdk.AccAddress{}, spAddr, spAddr, spAddr, blsPubKey, blsProof, coinPos, sdkerrors.ErrInvalidAddress},
		{"bad approval address", "a", "b", "c", "d", spAddr, spAddr, spAddr, spAddr, sdk.AccAddress{}, spAddr, spAddr, blsPubKey, blsProof, coinPos, sdkerrors.ErrInvalidAddress},
		{"bad gc address", "a", "b", "c", "d", spAddr, spAddr, spAddr, spAddr, spAddr, sdk.AccAddress{}, spAddr, blsPubKey, blsProof, coinPos, sdkerrors.ErrInvalidAddress},
		{"empty description", "", "", "", "", spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, blsPubKey, blsProof, coinPos, sdkerrors.ErrInvalidRequest},
		{"invalid bls key", "a", "b", "c", "d", spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, spAddr, deadbeefHex, blsProof, coinPos, sdkerrors.ErrInvalidPubKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := MsgCreateStorageProvider{
				Creator:            tt.creator.String(),
				Description:        NewDescription(tt.moniker, tt.identity, tt.website, tt.details),
				SpAddress:          tt.spAddress.String(),
				FundingAddress:     tt.fundingAddress.String(),
				SealAddress:        tt.sealAddress.String(),
				ApprovalAddress:    tt.approvalAddress.String(),
				GcAddress:          tt.gcAddress.String(),
				MaintenanceAddress: tt.maintenanceAddress.String(),
				BlsKey:             tt.blsKey,
				BlsProof:           tt.blsProof,
				Endpoint:           "http://127.0.0.1:9033",
				StorePrice:         math.LegacyZeroDec(),
				ReadPrice:          math.LegacyZeroDec(),
				Deposit:            tt.deposit,
			}
			err := msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgCreateStorageProvider_ValidateBasic_InvalidEndpointAndPrice(t *testing.T) {
	spAddr := sample.RandAccAddress()
	blsPubKey, blsProof := sample.RandBlsPubKeyAndBlsProof()
	base := MsgCreateStorageProvider{
		Creator:         spAddr.String(),
		SpAddress:       spAddr.String(),
		FundingAddress:  spAddr.String(),
		SealAddress:     spAddr.String(),
		ApprovalAddress: spAddr.String(),
		GcAddress:       spAddr.String(),
		Description:     NewDescription("m", "i", "w", "d"),
		BlsKey:          blsPubKey,
		BlsProof:        blsProof,
		Endpoint:        validEndpoint,
		ReadPrice:       math.LegacyZeroDec(),
		StorePrice:      math.LegacyZeroDec(),
		Deposit:         coinPos,
	}

	t.Run("invalid endpoint", func(t *testing.T) {
		msg := base
		msg.Endpoint = "http://127.0.0.1:9033/foo"
		err := msg.ValidateBasic()
		require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	})

	t.Run("nil read price", func(t *testing.T) {
		msg := base
		msg.ReadPrice = math.LegacyDec{}
		err := msg.ValidateBasic()
		require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	})

	t.Run("negative store price", func(t *testing.T) {
		msg := base
		msg.StorePrice = math.LegacyNewDec(-1)
		err := msg.ValidateBasic()
		require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	})
}

func TestNewMsgCreateStorageProvider(t *testing.T) {
	creator := sample.RandAccAddress()
	spAddr := sample.RandAccAddress()
	desc := NewDescription("m", "i", "w", "d")
	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	msg, err := NewMsgCreateStorageProvider(creator, spAddr, spAddr, spAddr, spAddr, spAddr, spAddr,
		desc, validEndpoint, coinPos, math.LegacyZeroDec(), 0, math.LegacyZeroDec(), blsKey, blsProof)
	require.NoError(t, err)
	require.Equal(t, creator.String(), msg.Creator)
	require.Equal(t, spAddr.String(), msg.SpAddress)
	require.Equal(t, desc, msg.Description)
	require.Equal(t, validEndpoint, msg.Endpoint)
	require.Equal(t, blsKey, msg.BlsKey)
	require.Equal(t, blsProof, msg.BlsProof)
}

func TestMsgCreateStorageProvider_RouteTypeSigners(t *testing.T) {
	creator := sample.RandAccAddress()
	msg := MsgCreateStorageProvider{Creator: creator.String()}

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCreateStorageProvider, msg.Type())
	require.Equal(t, []sdk.AccAddress{creator}, msg.GetSigners())
	require.NotEmpty(t, msg.GetSignBytes())

	bad := MsgCreateStorageProvider{Creator: badAddress}
	require.Panics(t, func() { bad.GetSigners() })
}

func TestMsgEditStorageProvider_ValidateBasic(t *testing.T) {
	pk1 := ed25519.GenPrivKey().PubKey()
	spAddr := sdk.AccAddress(pk1.Address())
	blsPubKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	tests := []struct {
		name, moniker, identity, website, details         string
		spAddress                                         sdk.AccAddress
		endpoint, sealAddress, approvalAddress, gcAddress string
		blsKey, blsProof                                  string
		err                                               error
	}{
		{"basic", "a1", "b1", "c1", "d1", spAddr, validEndpoint, "", "", "", blsPubKey, blsProof, nil},
		{"empty description", "", "", "", "", spAddr, "", "", "", "", blsPubKey, blsProof, sdkerrors.ErrInvalidRequest},
		{badSPAddressCase, "a1", "b1", "c1", "d1", sdk.AccAddress{}, "", "", "", "", blsPubKey, blsProof, sdkerrors.ErrInvalidAddress},
		{"invalid endpoint", "a1", "b1", "c1", "d1", spAddr, "/foo", "", "", "", blsPubKey, blsProof, sdkerrors.ErrInvalidRequest},
		{"bad seal address", "a1", "b1", "c1", "d1", spAddr, "", badAddress, "", "", blsPubKey, blsProof, sdkerrors.ErrInvalidAddress},
		{"bad approval address", "a1", "b1", "c1", "d1", spAddr, "", "", badAddress, "", blsPubKey, blsProof, sdkerrors.ErrInvalidAddress},
		{"bad gc address", "a1", "b1", "c1", "d1", spAddr, "", "", "", badAddress, blsPubKey, blsProof, sdkerrors.ErrInvalidAddress},
		{"bls key without proof", "a1", "b1", "c1", "d1", spAddr, "", "", "", "", blsPubKey, "", gnfderrors.ErrInvalidBlsSignature},
		{"invalid bls key with proof", "a1", "b1", "c1", "d1", spAddr, "", "", "", "", deadbeefHex, blsProof, sdkerrors.ErrInvalidPubKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := NewDescription(tt.moniker, tt.identity, tt.website, tt.details)
			msg := MsgEditStorageProvider{
				SpAddress:       tt.spAddress.String(),
				Endpoint:        tt.endpoint,
				Description:     &desc,
				SealAddress:     tt.sealAddress,
				ApprovalAddress: tt.approvalAddress,
				GcAddress:       tt.gcAddress,
				BlsKey:          tt.blsKey,
				BlsProof:        tt.blsProof,
			}
			err := msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgEditStorageProvider_ValidateBasic_MaintenanceAddress(t *testing.T) {
	spAddr := sample.RandAccAddress()
	desc := NewDescription("a1", "b1", "c1", "d1")

	t.Run("bad maintenance address", func(t *testing.T) {
		msg := MsgEditStorageProvider{SpAddress: spAddr.String(), Description: &desc, MaintenanceAddress: badAddress}
		err := msg.ValidateBasic()
		require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
	})

	t.Run("valid maintenance address", func(t *testing.T) {
		maintenance := sample.RandAccAddress()
		msg := MsgEditStorageProvider{SpAddress: spAddr.String(), Description: &desc, MaintenanceAddress: maintenance.String()}
		require.NoError(t, msg.ValidateBasic())
	})
}

func TestNewMsgEditStorageProvider(t *testing.T) {
	spAddr := sample.RandAccAddress()
	desc := NewDescription("m", "i", "w", "d")
	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	msg := NewMsgEditStorageProvider(spAddr, validEndpoint, &desc, spAddr, spAddr, spAddr, spAddr, blsKey, blsProof)
	require.Equal(t, spAddr.String(), msg.SpAddress)
	require.Equal(t, validEndpoint, msg.Endpoint)
	require.Equal(t, &desc, msg.Description)
	require.Equal(t, spAddr.String(), msg.SealAddress)
	require.Equal(t, blsKey, msg.BlsKey)
	require.Equal(t, blsProof, msg.BlsProof)
}

func TestMsgEditStorageProvider_RouteTypeSigners(t *testing.T) {
	spAddr := sample.RandAccAddress()
	msg := MsgEditStorageProvider{SpAddress: spAddr.String()}

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgEditStorageProvider, msg.Type())
	require.Equal(t, []sdk.AccAddress{spAddr}, msg.GetSigners())
	require.NotEmpty(t, msg.GetSignBytes())

	bad := MsgEditStorageProvider{SpAddress: badAddress}
	require.Panics(t, func() { bad.GetSigners() })
}

func TestMsgDeposit_ValidateBasic(t *testing.T) {
	pk1 := ed25519.GenPrivKey().PubKey()
	pk2 := ed25519.GenPrivKey().PubKey()
	fundAddr := sdk.AccAddress(pk1.Address())
	spAddr := sdk.AccAddress(pk2.Address())
	tests := []struct {
		name                   string
		fundAddress, spAddress sdk.AccAddress
		deposit                sdk.Coin
		err                    error
	}{
		{"basic", fundAddr, spAddr, coinPos, nil},
		{"bad creator address", sdk.AccAddress{}, spAddr, coinPos, sdkerrors.ErrInvalidAddress},
		{badSPAddressCase, fundAddr, sdk.AccAddress{}, coinPos, sdkerrors.ErrInvalidAddress},
		{"zero deposit", fundAddr, spAddr, coinZero, sdkerrors.ErrInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := MsgDeposit{Creator: tt.fundAddress.String(), SpAddress: tt.spAddress.String(), Deposit: tt.deposit}
			err := msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewMsgDeposit(t *testing.T) {
	fundAddr := sample.RandAccAddress()
	spAddr := sample.RandAccAddress()
	msg := NewMsgDeposit(fundAddr, spAddr, coinPos)
	require.Equal(t, fundAddr.String(), msg.Creator)
	require.Equal(t, spAddr.String(), msg.SpAddress)
	require.Equal(t, coinPos, msg.Deposit)
}

func TestMsgDeposit_RouteTypeSigners(t *testing.T) {
	fundAddr := sample.RandAccAddress()
	msg := MsgDeposit{Creator: fundAddr.String()}

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDeposit, msg.Type())
	require.Equal(t, []sdk.AccAddress{fundAddr}, msg.GetSigners())
	require.NotEmpty(t, msg.GetSignBytes())

	bad := MsgDeposit{Creator: badAddress}
	require.Panics(t, func() { bad.GetSigners() })
}

func TestMsgUpdateSpStoragePrice_ValidateBasic(t *testing.T) {
	spAddr := sample.RandAccAddress()

	tests := []struct {
		name       string
		spAddress  string
		readPrice  math.LegacyDec
		storePrice math.LegacyDec
		wantErr    bool
	}{
		{validCase, spAddr.String(), math.LegacyZeroDec(), math.LegacyZeroDec(), false},
		{badSPAddressCase, badAddress, math.LegacyZeroDec(), math.LegacyZeroDec(), true},
		{"nil read price", spAddr.String(), math.LegacyDec{}, math.LegacyZeroDec(), true},
		{"negative read price", spAddr.String(), math.LegacyNewDec(-1), math.LegacyZeroDec(), true},
		{"nil store price", spAddr.String(), math.LegacyZeroDec(), math.LegacyDec{}, true},
		{"negative store price", spAddr.String(), math.LegacyZeroDec(), math.LegacyNewDec(-1), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := MsgUpdateSpStoragePrice{SpAddress: tc.spAddress, ReadPrice: tc.readPrice, StorePrice: tc.storePrice}
			err := msg.ValidateBasic()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgUpdateSpStoragePrice_RouteTypeSigners(t *testing.T) {
	spAddr := sample.RandAccAddress()
	msg := MsgUpdateSpStoragePrice{SpAddress: spAddr.String()}

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgUpdateSpStoragePrice, msg.Type())
	require.Equal(t, []sdk.AccAddress{spAddr}, msg.GetSigners())
	require.NotEmpty(t, msg.GetSignBytes())

	bad := MsgUpdateSpStoragePrice{SpAddress: badAddress}
	require.Panics(t, func() { bad.GetSigners() })
}

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	authority := sample.RandAccAddress()

	tests := []struct {
		name      string
		authority string
		params    Params
		wantErr   bool
	}{
		{validCase, authority.String(), DefaultParams(), false},
		{"bad authority", badAddress, DefaultParams(), true},
		{"invalid params", authority.String(), Params{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := MsgUpdateParams{Authority: tc.authority, Params: tc.params}
			err := msg.ValidateBasic()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgUpdateParams_GetSignersAndSignBytes(t *testing.T) {
	authority := sample.RandAccAddress()
	msg := MsgUpdateParams{Authority: authority.String(), Params: DefaultParams()}
	require.Equal(t, []sdk.AccAddress{authority}, msg.GetSigners())
	require.NotEmpty(t, msg.GetSignBytes())

	// GetSigners swallows the parse error instead of panicking, unlike the
	// other message types in this file.
	bad := MsgUpdateParams{Authority: badAddress}
	require.Equal(t, []sdk.AccAddress{{}}, bad.GetSigners())
}

func TestNewMsgUpdateStorageProviderStatus(t *testing.T) {
	spAddr := sample.RandAccAddress()
	msg := NewMsgUpdateStorageProviderStatus(spAddr, STATUS_IN_MAINTENANCE, 100)
	require.Equal(t, spAddr.String(), msg.SpAddress)
	require.Equal(t, STATUS_IN_MAINTENANCE, msg.Status)
	require.Equal(t, int64(100), msg.Duration)
}

func TestMsgUpdateStorageProviderStatus_RouteTypeSigners(t *testing.T) {
	spAddr := sample.RandAccAddress()
	msg := MsgUpdateStorageProviderStatus{SpAddress: spAddr.String()}

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgUpdateStorageProviderStatus, msg.Type())
	require.Equal(t, []sdk.AccAddress{spAddr}, msg.GetSigners())
	require.NotEmpty(t, msg.GetSignBytes())

	bad := MsgUpdateStorageProviderStatus{SpAddress: badAddress}
	require.Panics(t, func() { bad.GetSigners() })
}

func TestMsgUpdateStorageProviderStatus_ValidateBasic(t *testing.T) {
	spAddr := sample.RandAccAddress()

	tests := []struct {
		name      string
		spAddress string
		status    Status
		duration  int64
		wantErr   bool
	}{
		{"valid in service", spAddr.String(), STATUS_IN_SERVICE, 0, false},
		{"valid in maintenance", spAddr.String(), STATUS_IN_MAINTENANCE, 100, false},
		{"bad address", badAddress, STATUS_IN_SERVICE, 0, true},
		{"disallowed status", spAddr.String(), STATUS_IN_JAILED, 0, true},
		{"maintenance without duration", spAddr.String(), STATUS_IN_MAINTENANCE, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := MsgUpdateStorageProviderStatus{SpAddress: tc.spAddress, Status: tc.status, Duration: tc.duration}
			err := msg.ValidateBasic()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateBlsKeyAndProof(t *testing.T) {
	validKey, validProof := sample.RandBlsPubKeyAndBlsProof()
	_, otherProof := sample.RandBlsPubKeyAndBlsProof()

	zeroKey := "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	zeroProof := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	tests := []struct {
		name    string
		key     string
		proof   string
		wantErr error
	}{
		{validCase, validKey, validProof, nil},
		{"bad hex key", "zz", validProof, sdkerrors.ErrInvalidPubKey},
		{"wrong length key", deadbeefHex, validProof, sdkerrors.ErrInvalidPubKey},
		{"right length but invalid key bytes", zeroKey, validProof, sdkerrors.ErrInvalidPubKey},
		{"bad hex proof", validKey, "zz", gnfderrors.ErrInvalidBlsSignature},
		{"wrong length proof", validKey, deadbeefHex, gnfderrors.ErrInvalidBlsSignature},
		{"right length but invalid proof bytes", validKey, zeroProof, sdkerrors.ErrorInvalidSigner},
		{"verification fails for mismatched key and proof", validKey, otherProof, sdkerrors.ErrorInvalidSigner},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBlsKeyAndProof(tc.key, tc.proof)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
