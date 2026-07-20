package storage

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/mocachain/moca/v2/contracts"
	"github.com/mocachain/moca/v2/precompiles/types"
	gtypes "github.com/mocachain/moca/v2/types"
)

const (
	// CreateBucketEventName is the event emitted on a createBucket transaction.
	CreateBucketEventName = "CreateBucket"
	// UpdateBucketInfoEventName is the event emitted on an updateBucketInfo transaction.
	UpdateBucketInfoEventName = "UpdateBucketInfo"
	// DeleteBucketEventName is the event emitted on a deleteBucket transaction.
	DeleteBucketEventName = "DeleteBucket"
	// DiscontinueBucketEventName is the event emitted on a discontinueBucket transaction.
	DiscontinueBucketEventName = "DiscontinueBucket"
	// MigrateBucketEventName is the event emitted on a migrateBucket transaction.
	MigrateBucketEventName = "MigrateBucket"
	// CompleteMigrateBucketEventName is the event emitted on a completeMigrateBucket transaction.
	CompleteMigrateBucketEventName = "CompleteMigrateBucket"
	// RejectMigrateBucketEventName is the event emitted on a rejectMigrateBucket transaction.
	RejectMigrateBucketEventName = "RejectMigrateBucket"
	// CancelMigrateBucketEventName is the event emitted on a cancelMigrateBucket transaction.
	CancelMigrateBucketEventName = "CancelMigrateBucket"
	// SetBucketFlowRateLimitEventName is the event emitted on a setBucketFlowRateLimit transaction.
	SetBucketFlowRateLimitEventName = "SetBucketFlowRateLimit"
	// CreateObjectEventName is the event emitted on a createObject transaction.
	CreateObjectEventName = "CreateObject"
	// CopyObjectEventName is the event emitted on a copyObject transaction.
	CopyObjectEventName = "CopyObject"
	// DeleteObjectEventName is the event emitted on a deleteObject transaction.
	DeleteObjectEventName = "DeleteObject"
	// CancelCreateObjectEventName is the event emitted on a cancelCreateObject transaction.
	CancelCreateObjectEventName = "CancelCreateObject"
	// SealObjectEventName is the event emitted on a sealObject transaction.
	SealObjectEventName = "SealObject"
	// SealObjectV2EventName is the event emitted on a sealObjectV2 transaction.
	SealObjectV2EventName = "SealObjectV2"
	// RejectSealObjectEventName is the event emitted on a rejectSealObject transaction.
	RejectSealObjectEventName = "RejectSealObject"
	// DelegateCreateObjectEventName is the event emitted on a delegateCreateObject transaction.
	DelegateCreateObjectEventName = "DelegateCreateObject"
	// DelegateUpdateObjectContentEventName is the event emitted on a delegateUpdateObjectContent transaction.
	DelegateUpdateObjectContentEventName = "DelegateUpdateObjectContent"
	// UpdateObjectInfoEventName is the event emitted on an updateObjectInfo transaction.
	UpdateObjectInfoEventName = "UpdateObjectInfo"
	// UpdateObjectContentEventName is the event emitted on an updateObjectContent transaction.
	UpdateObjectContentEventName = "UpdateObjectContent"
	// DiscontinueObjectEventName is the event emitted on a discontinueObject transaction.
	DiscontinueObjectEventName = "DiscontinueObject"
	// CreateGroupEventName is the event emitted on a createGroup transaction.
	CreateGroupEventName = "CreateGroup"
	// UpdateGroupEventName is the event emitted on an updateGroup transaction.
	UpdateGroupEventName = "UpdateGroup"
	// UpdateGroupExtraEventName is the event emitted on an updateGroupExtra transaction.
	UpdateGroupExtraEventName = "UpdateGroupExtra"
	// DeleteGroupEventName is the event emitted on a deleteGroup transaction.
	DeleteGroupEventName = "DeleteGroup"
	// LeaveGroupEventName is the event emitted on a leaveGroup transaction.
	LeaveGroupEventName = "LeaveGroup"
	// RenewGroupMemberEventName is the event emitted on a renewGroupMember transaction.
	RenewGroupMemberEventName = "RenewGroupMember"
	// SetTagEventName is the event emitted on a setTag transaction.
	SetTagEventName = "SetTag"
	// PutPolicyEventName is the event emitted on a putPolicy transaction.
	PutPolicyEventName = "PutPolicy"
	// DeletePolicyEventName is the event emitted on a deletePolicy transaction.
	DeletePolicyEventName = "DeletePolicy"
	// ToggleSPAsDelegatedAgentEventName is the event emitted on a toggleSPAsDelegatedAgent transaction.
	ToggleSPAsDelegatedAgentEventName = "ToggleSPAsDelegatedAgent"
	// CancelUpdateObjectContentEventName is the event emitted on a cancelUpdateObjectContent transaction.
	CancelUpdateObjectContentEventName = "CancelUpdateObjectContent"
	// TransferEventName is the ERC721 Transfer event mirrored on bucket/object/group NFT mints.
	TransferEventName = "Transfer"
)

// EmitCreateBucketEvent emits the CreateBucket event with the caller, payment and
// primary SP addresses as indexed topics and the bucket id as data.
func (p Precompile) EmitCreateBucketEvent(evm *vm.EVM, caller, paymentAddress, primarySpAddress common.Address, bucketID *big.Int) error {
	return p.AddLog(evm, MustEvent(CreateBucketEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			common.BytesToHash(paymentAddress.Bytes()),
			common.BytesToHash(primarySpAddress.Bytes()),
		},
		bucketID)
}

// EmitUpdateBucketInfoEvent emits the UpdateBucketInfo event with the caller, the
// keccak256 bucket name and the payment address as indexed topics and visibility as data.
func (p Precompile) EmitUpdateBucketInfoEvent(evm *vm.EVM, caller common.Address, bucketName string, paymentAddress common.Address, visibility uint8) error {
	return p.AddLog(evm, MustEvent(UpdateBucketInfoEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(bucketName)),
			common.BytesToHash(paymentAddress.Bytes()),
		},
		visibility)
}

// EmitDeleteBucketEvent emits the DeleteBucket event with the caller as the sole indexed topic.
func (p Precompile) EmitDeleteBucketEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(DeleteBucketEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitDiscontinueBucketEvent emits the DiscontinueBucket event with the caller and the
// keccak256 bucket name as indexed topics.
func (p Precompile) EmitDiscontinueBucketEvent(evm *vm.EVM, caller common.Address, bucketName string) error {
	return p.AddLog(evm, MustEvent(DiscontinueBucketEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(bucketName)),
		})
}

// EmitMigrateBucketEvent emits the MigrateBucket event with the caller and the
// keccak256 bucket name as indexed topics.
func (p Precompile) EmitMigrateBucketEvent(evm *vm.EVM, caller common.Address, bucketName string) error {
	return p.AddLog(evm, MustEvent(MigrateBucketEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(bucketName)),
		})
}

// EmitCompleteMigrateBucketEvent emits the CompleteMigrateBucket event with the caller
// and the keccak256 bucket name as indexed topics.
func (p Precompile) EmitCompleteMigrateBucketEvent(evm *vm.EVM, caller common.Address, bucketName string) error {
	return p.AddLog(evm, MustEvent(CompleteMigrateBucketEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(bucketName)),
		})
}

// EmitRejectMigrateBucketEvent emits the RejectMigrateBucket event with the caller as the sole indexed topic.
func (p Precompile) EmitRejectMigrateBucketEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(RejectMigrateBucketEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitCancelMigrateBucketEvent emits the CancelMigrateBucket event with the caller as the sole indexed topic.
func (p Precompile) EmitCancelMigrateBucketEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(CancelMigrateBucketEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitSetBucketFlowRateLimitEvent emits the SetBucketFlowRateLimit event with the caller as the sole indexed topic.
func (p Precompile) EmitSetBucketFlowRateLimitEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(SetBucketFlowRateLimitEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitCreateObjectEvent emits the CreateObject event with the caller as an indexed
// topic and the object id as data.
func (p Precompile) EmitCreateObjectEvent(evm *vm.EVM, caller common.Address, objectID *big.Int) error {
	return p.AddLog(evm, MustEvent(CreateObjectEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())},
		objectID)
}

// EmitCopyObjectEvent emits the CopyObject event with the caller as the sole indexed topic.
func (p Precompile) EmitCopyObjectEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(CopyObjectEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitDeleteObjectEvent emits the DeleteObject event with the caller as the sole indexed topic.
func (p Precompile) EmitDeleteObjectEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(DeleteObjectEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitCancelCreateObjectEvent emits the CancelCreateObject event with the caller as the sole indexed topic.
func (p Precompile) EmitCancelCreateObjectEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(CancelCreateObjectEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitSealObjectEvent emits the SealObject event with the caller as the sole indexed topic.
func (p Precompile) EmitSealObjectEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(SealObjectEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitSealObjectV2Event emits the SealObjectV2 event with the caller as the sole indexed topic.
func (p Precompile) EmitSealObjectV2Event(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(SealObjectV2EventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitRejectSealObjectEvent emits the RejectSealObject event with the caller and the
// keccak256 object name as indexed topics.
func (p Precompile) EmitRejectSealObjectEvent(evm *vm.EVM, caller common.Address, objectName string) error {
	return p.AddLog(evm, MustEvent(RejectSealObjectEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(objectName)),
		})
}

// EmitDelegateCreateObjectEvent emits the DelegateCreateObject event with the caller
// and the keccak256 object name as indexed topics.
func (p Precompile) EmitDelegateCreateObjectEvent(evm *vm.EVM, caller common.Address, objectName string) error {
	return p.AddLog(evm, MustEvent(DelegateCreateObjectEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(objectName)),
		})
}

// EmitDelegateUpdateObjectContentEvent emits the DelegateUpdateObjectContent event with
// the caller and the keccak256 object name as indexed topics.
func (p Precompile) EmitDelegateUpdateObjectContentEvent(evm *vm.EVM, caller common.Address, objectName string) error {
	return p.AddLog(evm, MustEvent(DelegateUpdateObjectContentEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(objectName)),
		})
}

// EmitUpdateObjectInfoEvent emits the UpdateObjectInfo event with the caller as the sole indexed topic.
func (p Precompile) EmitUpdateObjectInfoEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(UpdateObjectInfoEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitUpdateObjectContentEvent emits the UpdateObjectContent event with the caller and
// the keccak256 object name as indexed topics.
func (p Precompile) EmitUpdateObjectContentEvent(evm *vm.EVM, caller common.Address, objectName string) error {
	return p.AddLog(evm, MustEvent(UpdateObjectContentEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(objectName)),
		})
}

// EmitDiscontinueObjectEvent emits the DiscontinueObject event with the caller and the
// keccak256 bucket name as indexed topics.
func (p Precompile) EmitDiscontinueObjectEvent(evm *vm.EVM, caller common.Address, bucketName string) error {
	return p.AddLog(evm, MustEvent(DiscontinueObjectEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(bucketName)),
		})
}

// EmitCreateGroupEvent emits the CreateGroup event with the caller as an indexed topic
// and the group id as data.
func (p Precompile) EmitCreateGroupEvent(evm *vm.EVM, caller common.Address, groupID *big.Int) error {
	return p.AddLog(evm, MustEvent(CreateGroupEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())},
		groupID)
}

// EmitUpdateGroupEvent emits the UpdateGroup event with the caller as the sole indexed topic.
func (p Precompile) EmitUpdateGroupEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(UpdateGroupEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitUpdateGroupExtraEvent emits the UpdateGroupExtra event with the caller as the sole indexed topic.
func (p Precompile) EmitUpdateGroupExtraEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(UpdateGroupExtraEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitDeleteGroupEvent emits the DeleteGroup event with the caller as the sole indexed topic.
func (p Precompile) EmitDeleteGroupEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(DeleteGroupEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitLeaveGroupEvent emits the LeaveGroup event with the caller and the keccak256
// group name as indexed topics.
func (p Precompile) EmitLeaveGroupEvent(evm *vm.EVM, caller common.Address, groupName string) error {
	return p.AddLog(evm, MustEvent(LeaveGroupEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(groupName)),
		})
}

// EmitRenewGroupMemberEvent emits the RenewGroupMember event with the caller as the sole indexed topic.
func (p Precompile) EmitRenewGroupMemberEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(RenewGroupMemberEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitSetTagEvent emits the SetTag event with the caller as the sole indexed topic.
func (p Precompile) EmitSetTagEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(SetTagEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitPutPolicyEvent emits the PutPolicy event with the caller as the sole indexed topic.
func (p Precompile) EmitPutPolicyEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(PutPolicyEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitDeletePolicyEvent emits the DeletePolicy event with the caller as the sole indexed topic.
func (p Precompile) EmitDeletePolicyEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(DeletePolicyEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitToggleSPAsDelegatedAgentEvent emits the ToggleSPAsDelegatedAgent event with the caller as the sole indexed topic.
func (p Precompile) EmitToggleSPAsDelegatedAgentEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(ToggleSPAsDelegatedAgentEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitCancelUpdateObjectContentEvent emits the CancelUpdateObjectContent event with the
// caller and the keccak256 object name as indexed topics.
func (p Precompile) EmitCancelUpdateObjectContentEvent(evm *vm.EVM, caller common.Address, objectName string) error {
	return p.AddLog(evm, MustEvent(CancelUpdateObjectContentEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			crypto.Keccak256Hash([]byte(objectName)),
		})
}

// EmitBucketTransferEvent mirrors the bucket-NFT mint as an ERC721 Transfer log on the
// bucket token contract: from the zero address to the owner, with the token id.
func (p Precompile) EmitBucketTransferEvent(evm *vm.EVM, owner string, tokenID *big.Int) error {
	return p.emitNFTTransferEvent(evm, contracts.BucketERC721TokenAddress, owner, tokenID)
}

// EmitObjectTransferEvent mirrors the object-NFT mint as an ERC721 Transfer log on the
// object token contract: from the zero address to the owner, with the token id.
func (p Precompile) EmitObjectTransferEvent(evm *vm.EVM, owner string, tokenID *big.Int) error {
	return p.emitNFTTransferEvent(evm, contracts.ObjectERC721TokenAddress, owner, tokenID)
}

// EmitGroupTransferEvent mirrors the group-NFT mint as an ERC721 Transfer log on the
// group token contract: from the zero address to the owner, with the token id.
func (p Precompile) EmitGroupTransferEvent(evm *vm.EVM, owner string, tokenID *big.Int) error {
	return p.emitNFTTransferEvent(evm, contracts.GroupERC721TokenAddress, owner, tokenID)
}

// emitNFTTransferEvent emits the ERC721 Transfer event (from, to, tokenId all indexed)
// on the given NFT token contract address via AddOtherLog. The mint is always from the
// empty EVM address; the token id is packed as its big-endian bytes, matching the
// original inline emission.
func (p Precompile) emitNFTTransferEvent(evm *vm.EVM, tokenContract common.Address, owner string, tokenID *big.Int) error {
	return p.AddOtherLog(evm, MustEvent(TransferEventName), tokenContract,
		[]common.Hash{
			common.BytesToHash(common.HexToAddress(gtypes.EmptyEvmAddress).Bytes()),
			common.BytesToHash(common.HexToAddress(owner).Bytes()),
			common.BytesToHash(tokenID.Bytes()),
		})
}

// AddLog packs the given event and appends it to the StateDB logs at the precompile address.
func (p Precompile) AddLog(evm *vm.EVM, event abi.Event, topics []common.Hash, args ...interface{}) error {
	data, packedTopics, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      packedTopics,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}

// AddOtherLog packs the given event and appends it to the StateDB logs at a different
// contract address than the precompile (used for the ERC721 NFT Transfer mirror logs).
func (p Precompile) AddOtherLog(evm *vm.EVM, event abi.Event, address common.Address, topics []common.Hash, args ...interface{}) error {
	data, packedTopics, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     address,
		Topics:      packedTopics,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}
