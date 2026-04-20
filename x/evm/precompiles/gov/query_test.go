package gov

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/encoding"
)

func TestOutputsProposal_HistoricalCrisisMessageCompatibility(t *testing.T) {
	t.Parallel()

	msg := crisistypes.NewMsgVerifyInvariant(
		sdk.AccAddress(bytes.Repeat([]byte{1}, 20)),
		"bank",
		"supply",
	)
	anyMsg, err := codectypes.NewAnyWithValue(msg)
	require.NoError(t, err)

	proposal := govv1.Proposal{
		Id:               42,
		Messages:         []*codectypes.Any{anyMsg},
		Status:           govv1.StatusDepositPeriod,
		FinalTallyResult: &govv1.TallyResult{},
		SubmitTime:       ptr(time.Unix(10, 0)),
		DepositEndTime:   ptr(time.Unix(20, 0)),
		Metadata:         "meta",
		Title:            "title",
		Summary:          "summary",
		Proposer:         "0x0000000000000000000000000000000000000001",
	}

	writerRegistry := codectypes.NewInterfaceRegistry()
	crisistypes.RegisterInterfaces(writerRegistry)
	writerCodec := codec.NewProtoCodec(writerRegistry)

	bz, err := writerCodec.Marshal(&proposal)
	require.NoError(t, err)

	readerCodec := encoding.MakeConfig().Codec
	var decoded govv1.Proposal
	require.NoError(t, readerCodec.Unmarshal(bz, &decoded), "historical crisis messages must stay decodable")

	msgs, err := decoded.GetMsgs()
	require.NoError(t, err, "proposal.GetMsgs should succeed for historical crisis proposals")
	require.Len(t, msgs, 1)

	out := OutputsProposal(decoded)
	require.Equal(t, uint64(42), out.Id)
	require.Equal(t, "meta", out.Metadata)
	require.Len(t, out.Messages, 1)
	require.True(t, strings.Contains(out.Messages[0], "MsgVerifyInvariant"), out.Messages[0])
}

func ptr[T any](v T) *T {
	return &v
}
