package storage_test

import (
	"bytes"
	"encoding/base64"
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/suite"

	"github.com/mocachain/moca/v2/contracts"
	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/testutil/sample"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// ObjectNFTLifecycleTestSuite seals objects and updates their content through the
// storage precompile on a chain that holds the object NFT contract from genesis,
// and reads the token back from that contract. NFTMirrorLogTestSuite stops at the
// Transfer logs the precompile mirrors; this suite checks the token itself. It
// reuses DeleteGCBookkeepingTestSuite's fixtures without running that suite's own
// tests again.
type ObjectNFTLifecycleTestSuite struct {
	suite.Suite
	env           DeleteGCBookkeepingTestSuite
	secondaryKeys []*bls.PrivateKey
}

func TestObjectNFTLifecycleTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectNFTLifecycleTestSuite))
}

const (
	lifecycleBucketID = 601
	lifecycleObjectID = 602
	lifecyclePayload  = 1024
)

// SetupTest makes the test account the seal address of the bucket's primary SP and
// gives the global virtual group secondary SPs with BLS keys, so seals go through
// the same signature check as on a live chain.
func (s *ObjectNFTLifecycleTestSuite) SetupTest() {
	s.env.SetT(s.T())
	s.env.SetupTest()
	s.env.setupVirtualGroups()
	s.Require().NoError(s.env.app.VirtualgroupKeeper.SetParams(s.env.ctx, vgtypes.DefaultParams()))

	primary := &sptypes.StorageProvider{
		Id:              testGVGFamilyID,
		OperatorAddress: sample.RandAccAddress().String(),
		SealAddress:     sdk.AccAddress(s.env.address.Bytes()).String(),
		Status:          sptypes.STATUS_IN_SERVICE,
	}
	s.env.app.SpKeeper.SetStorageProvider(s.env.ctx, primary)
	s.env.app.SpKeeper.SetStorageProviderBySealAddr(s.env.ctx, primary)

	s.secondaryKeys = nil
	secondaryIDs := make([]uint32, s.env.app.StorageKeeper.GetExpectSecondarySPNumForECObject(s.env.ctx, s.env.resourceTime()))
	for i := range secondaryIDs {
		key, err := bls.GenerateBlsKey()
		s.Require().NoError(err)
		s.secondaryKeys = append(s.secondaryKeys, key)
		secondaryIDs[i] = testGVGFamilyID + 1 + uint32(i)
		s.env.app.SpKeeper.SetStorageProvider(s.env.ctx, &sptypes.StorageProvider{
			Id:              secondaryIDs[i],
			OperatorAddress: sample.RandAccAddress().String(),
			BlsKey:          key.PublicKey().Marshal(),
			Status:          sptypes.STATUS_IN_SERVICE,
		})
	}
	s.env.app.VirtualgroupKeeper.SetGVG(s.env.ctx, &vgtypes.GlobalVirtualGroup{
		Id:                    testGVGID,
		FamilyId:              testGVGFamilyID,
		PrimarySpId:           testGVGFamilyID,
		SecondarySpIds:        secondaryIDs,
		TotalDeposit:          sdkmath.NewIntWithDecimal(1, 24),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
}

// TestUpdateWithContent_KeepsNFT replaces an object's content with new content. The
// object keeps the token minted on its first seal; the contract refuses a second
// mint of a token, so a seal that minted again would fail.
func (s *ObjectNFTLifecycleTestSuite) TestUpdateWithContent_KeepsNFT() {
	bucketName, objectName, id := s.sealedObject("nft-keep")

	s.env.callPrecompile(s.updateContentInput(bucketName, objectName, 2*lifecyclePayload, testChecksums(2)))
	s.requireObjectNFT(id)

	s.seal(bucketName, objectName, testChecksums(2))
	s.Require().Equal(uint64(2*lifecyclePayload), s.objectInfo(bucketName, objectName).PayloadSize)
	s.requireObjectNFT(id)
}

// TestUpdateToEmptyAndBack burns the token when an update removes the object's
// content and mints it again when a later update brings content back.
func (s *ObjectNFTLifecycleTestSuite) TestUpdateToEmptyAndBack() {
	bucketName, objectName, id := s.sealedObject("nft-empty")

	s.env.callPrecompile(s.updateContentInput(bucketName, objectName, 0, testChecksums(2)))
	s.Require().Zero(s.objectInfo(bucketName, objectName).PayloadSize)
	s.requireNoObjectNFT(id)

	s.env.callPrecompile(s.updateContentInput(bucketName, objectName, lifecyclePayload, testChecksums(3)))
	s.requireNoObjectNFT(id)
	s.seal(bucketName, objectName, testChecksums(3))
	s.requireObjectNFT(id)
}

// TestFailedUpdateToEmpty_RevertsBurn sends an update that removes the object's
// content in an EVM transaction that runs out of gas after the token was burned,
// then sends it again with enough gas. The failed transaction must leave the
// object and its token as they were, burn included.
func (s *ObjectNFTLifecycleTestSuite) TestFailedUpdateToEmpty_RevertsBurn() {
	bucketName, objectName, id := s.sealedObject("nft-oog")
	input := s.updateContentInput(bucketName, objectName, 0, testChecksums(2))

	// The least gas the update succeeds with, probed on branches of the state. One
	// unit less runs the update up to its last gas charge, which comes after the
	// contract call that burns the token.
	succeeds := func(gas uint64) bool {
		branch, _ := s.env.ctx.CacheContext()
		res, err := s.applyWithGas(branch, input, gas)
		return err == nil && !res.Failed()
	}
	failing, need := uint64(0), uint64(10_000_000)
	s.Require().True(succeeds(need), "the update never succeeded")
	for failing+1 < need {
		if mid := failing + (need-failing)/2; succeeds(mid) {
			need = mid
		} else {
			failing = mid
		}
	}

	res, err := s.applyWithGas(s.env.ctx, input, need-1)
	s.Require().NoError(err)
	s.Require().True(res.Failed())
	s.Require().Contains(res.VmError, vm.ErrOutOfGas.Error())
	s.Require().Equal(uint64(lifecyclePayload), s.objectInfo(bucketName, objectName).PayloadSize)
	s.requireObjectNFT(id)

	res, err = s.applyWithGas(s.env.ctx, input, need)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), res.VmError)
	s.Require().Zero(s.objectInfo(bucketName, objectName).PayloadSize)
	s.requireNoObjectNFT(id)
}

// sealedObject creates an object with content in a new bucket and seals it, which
// mints the object's token.
func (s *ObjectNFTLifecycleTestSuite) sealedObject(name string) (bucketName, objectName string, id sdkmath.Uint) {
	bucketName, objectName, id = name+"-bucket", name+"-object", sdkmath.NewUint(lifecycleObjectID)
	s.env.createBucketWithPolicy(bucketName, sdkmath.NewUint(lifecycleBucketID), nil)
	s.env.createObjectWithStatus(bucketName, objectName, id, storagetypes.OBJECT_STATUS_CREATED, lifecyclePayload)
	s.requireNoObjectNFT(id)

	s.seal(bucketName, objectName, testChecksums(1))
	s.requireObjectNFT(id)
	return bucketName, objectName, id
}

// seal seals the object's pending content with the secondary SPs' aggregated BLS
// signature over it, as the primary SP does.
func (s *ObjectNFTLifecycleTestSuite) seal(bucketName, objectName string, checksums [][]byte) {
	signHash := storagetypes.NewSecondarySpSealObjectSignDoc(s.env.ctx.ChainID(), testGVGID,
		s.objectInfo(bucketName, objectName).Id, storagetypes.GenerateHash(checksums)).GetBlsSignHash()
	sigs := make(bls.Signatures, 0, len(s.secondaryKeys))
	for _, key := range s.secondaryKeys {
		sig, err := key.Sign(signHash[:], votepool.DST)
		s.Require().NoError(err)
		sigs = append(sigs, sig)
	}
	aggSig, err := sigs.Aggregate().Marshal()
	s.Require().NoError(err)

	s.env.callPrecompile(s.env.packed(storage.SealObjectV2MethodName, bucketName, objectName, testGVGID,
		base64.StdEncoding.EncodeToString(aggSig), encodeChecksums(checksums)))
}

func (s *ObjectNFTLifecycleTestSuite) updateContentInput(bucketName, objectName string, payloadSize uint64, checksums [][]byte) []byte {
	return s.env.packed(storage.UpdateObjectContentMethodName, bucketName, objectName, payloadSize,
		"application/octet-stream", encodeChecksums(checksums))
}

// applyWithGas sends input to the storage precompile in an EVM transaction from the
// test account with the given gas limit, metered from zero as a new transaction is.
func (s *ObjectNFTLifecycleTestSuite) applyWithGas(ctx sdk.Context, input []byte, gasLimit uint64) (*evmtypes.MsgEthereumTxResponse, error) {
	ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	nonce, err := s.env.app.AccountKeeper.GetSequence(ctx, s.env.address.Bytes())
	s.Require().NoError(err)
	to := storage.GetAddress()
	msg := core.Message{
		From:      s.env.address,
		To:        &to,
		Nonce:     nonce,
		Value:     big.NewInt(0),
		GasLimit:  gasLimit,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      input,
	}
	stateDB := statedb.New(ctx, s.env.app.EvmKeeper, statedb.NewEmptyTxConfig())
	return s.env.app.EvmKeeper.ApplyMessage(ctx, stateDB, msg, nil, true, false, true)
}

func (s *ObjectNFTLifecycleTestSuite) objectInfo(bucketName, objectName string) *storagetypes.ObjectInfo {
	info, found := s.env.app.StorageKeeper.GetObjectInfo(s.env.ctx, bucketName, objectName)
	s.Require().True(found)
	return info
}

// requireObjectNFT asserts that the test account holds the object's token and that
// its URI is the contract's base URI followed by the token id.
func (s *ObjectNFTLifecycleTestSuite) requireObjectNFT(id sdkmath.Uint) {
	owner, err := s.callObjectNFT("ownerOf", id.BigInt())
	s.Require().NoError(err)
	s.Require().Equal(s.env.address, owner)
	baseURI, err := s.callObjectNFT("baseURI")
	s.Require().NoError(err)
	uri, err := s.callObjectNFT("tokenURI", id.BigInt())
	s.Require().NoError(err)
	s.Require().Equal(baseURI.(string)+id.String(), uri)
}

func (s *ObjectNFTLifecycleTestSuite) requireNoObjectNFT(id sdkmath.Uint) {
	_, err := s.callObjectNFT("ownerOf", id.BigInt())
	s.Require().ErrorContains(err, "execution reverted", "token %s should not exist", id)
}

// callObjectNFT reads from the object NFT contract on a branch of the state, since
// a reverted call charges the context's whole gas limit.
func (s *ObjectNFTLifecycleTestSuite) callObjectNFT(method string, args ...interface{}) (interface{}, error) {
	branch, _ := s.env.ctx.CacheContext()
	contractABI := contracts.ERC721NonTransferableContract.ABI
	res, err := s.env.app.StorageKeeper.CallEVM(branch.WithGasMeter(storetypes.NewInfiniteGasMeter()), contractABI,
		contracts.ObjectControlHubAddress, contracts.ObjectERC721TokenAddress, false, method, args...)
	if err != nil {
		return nil, err
	}
	out, err := contractABI.Unpack(method, res.Ret)
	s.Require().NoError(err)
	return out[0], nil
}

// testChecksums returns a distinct checksum list for each n: one for the whole
// object and one per secondary SP.
func testChecksums(n byte) [][]byte {
	checksums := make([][]byte, 1+storagetypes.DefaultRedundantDataChunkNum+storagetypes.DefaultRedundantParityChunkNum)
	for i := range checksums {
		checksums[i] = bytes.Repeat([]byte{n<<4 | byte(i)}, 32)
	}
	return checksums
}

func encodeChecksums(checksums [][]byte) []string {
	encoded := make([]string, len(checksums))
	for i, c := range checksums {
		encoded[i] = base64.StdEncoding.EncodeToString(c)
	}
	return encoded
}
