# EVM e2e suite

This package drives moca's storage, permission, payment, storage-provider and
virtualgroup modules through their EVM precompiles: every message a test
exercises goes out as a real signed EVM transaction against a live `localup`
chain, and results are read back over the node's gRPC query services. It
replaced the Cosmos-message suite that used to live here (and in `e2e/core`),
which signed through the EIP-712 path the chain no longer supports and had been
failing for a long time.

## Running it

```bash
make build
cd deployment/localup
bash localup.sh all 1 7          # 1 validator, 7 storage providers
bash localup.sh export_sps 1 7   # writes .local/sp_export.json with the SP role keys
cd ../..
go test ./e2e/tests/... -v -timeout 10m
```

The harness (`evm_harness_test.go`) assumes the chain `localup.sh` produces:
EVM JSON-RPC on `127.0.0.1:8545`, gRPC on `localhost:9090`, chain id 5151,
the well-known pre-funded dev account, and the SP keys in `sp_export.json`.
It also relies on genesis values `localup.sh` pins for test speed:

| Param | localup value | Default | Used by |
| --- | --- | --- | --- |
| `x/sp` `update_global_price_interval` | 1 s | monthly | `TestStorageProviderPriceEvmFlow` |
| `x/sp` `update_price_disallowed_days` | 0 | 2 | `TestStorageProviderPriceEvmFlow` |
| `x/payment` `withdraw_time_lock_duration` | 10 s | 1 day | `TestPaymentWithdrawDelayedEvmFlow` |

`.github/workflows/e2e-localup.yml` runs exactly this on every PR and push to
`main`. The suite is safe to re-run against the same chain: tests create their
own accounts, buckets and groups, and the SP price test doubles the quotes of
the same four SPs each run, which keeps its median arithmetic valid.

## Inventory of the retired suite

The retired suite held 119 tests across nine files. Every one is listed below
with what covers it now, or the reason it is not covered yet.

- **ported**: the scenario's functional path is exercised by the named test.
    Details that only existed because of the Cosmos-message transport (for
    example two messages in one transaction) are not reproduced.
- **partial**: the named part of the scenario is exercised; the rest is stated.
- **deferred**: not reproduced, for the stated reason:
    - *BLS seal*: needs a non-trivial object sealed with a BLS aggregate
      signature from the secondary SPs. The harness has no sign-doc or
      aggregation tooling for that yet. It also gates the challenge module,
      payment settlement on stored objects and `completeMigrateBucket`.
    - *governance*: needs a module parameter changed through a governance
      proposal mid-test. The harness has no governance path and no validator
      key.
    - *SP exit/swap*: needs the multi-SP `swapOut`/`spExit` lifecycle modelled
      end to end.
    - *no precompile*: the message has no EVM-facing method.
    - *portable*: nothing blocks it; it has not been ported yet.

### challenge_test.go (7)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestSubmit | deferred | BLS seal: challenges target a sealed object |
| TestNormalAttest | deferred | BLS seal |
| TestHeartbeatAttest | deferred | BLS seal |
| TestFailedAttest_ChallengeExpired | deferred | BLS seal |
| TestFailedAttest_ExceedMaxSlashAmount | deferred | BLS seal, governance |
| TestEndBlock | deferred | BLS seal: random challenges need sealed objects |
| TestUpdateChallengerParams | deferred | governance |

### gensp_test.go (1)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestGenStorageProvider | ported | `TestGenesisStorageProviderEvmFlow` |

### payment_test.go (24)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestQueryPaymentAccounts | ported | `TestPaymentAccountEvmFlow` |
| TestCreatePaymentAccount | ported | `TestPaymentAccountEvmFlow` |
| TestVersionedParams_SealObjectAfterReserveTimeChange | deferred | governance, BLS seal |
| TestVersionedParams_DeleteBucketAfterValidatorTaxRateChange | deferred | governance, BLS seal |
| TestVersionedParams_DeleteObjectAfterReserveTimeChange | deferred | governance, BLS seal |
| TestDeposit_ActiveAccount | partial | deposit path in `TestPaymentDepositEvmFlow`; settling a streaming payment account on deposit is portable |
| TestDeposit_FromBankAccount | ported | `TestPaymentDepositEvmFlow` |
| TestDeposit_ResumeInOneBlock | deferred | portable: needs the frozen-account lifecycle (stream runs dry, forced settle) modelled |
| TestDeposit_ResumeInBlocks | deferred | governance, frozen-account lifecycle |
| TestAutoSettle_InOneBlock | deferred | portable: frozen-account lifecycle |
| TestAutoSettle_InBlocks | deferred | governance, frozen-account lifecycle |
| TestWithdraw | ported | `TestPaymentWithdrawImmediateEvmFlow`: overdraw refused, bank balance delta checked |
| TestWithdrawDelayed | ported | `TestPaymentWithdrawDelayedEvmFlow`: full lifecycle; the lock is pinned to 10 s in localup genesis instead of shortened through governance |
| TestVirtualGroup_Settle | deferred | BLS seal, SP exit/swap |
| TestDiscontinue_InOneBlock_WithoutPriceChange | partial | discontinue path in `TestStorageDiscontinueObjectEvmFlow` and `TestStorageDiscontinueBucketEvmFlow`; settlement of stored objects deferred: BLS seal |
| TestDiscontinue_InOneBlock_WithPriceChange | partial | as above |
| TestDiscontinue_InBlocks_WithoutPriceChange | partial | as above |
| TestDiscontinue_InBlocks_WithPriceChange | partial | as above |
| TestDiscontinue_InBlocks_WithPriceChangeReserveTimeChange | deferred | BLS seal, governance |
| TestDiscontinue_InBlocks_WithPriceChangeReserveTimeChange_FrozenAccount | deferred | BLS seal, governance |
| TestDiscontinue_MultiObjects | partial | as TestDiscontinue_InOneBlock_WithoutPriceChange |
| TestDiscontinue_MultiBuckets | partial | as TestDiscontinue_InOneBlock_WithoutPriceChange |
| TestDeposit_FrozenAccount_NetflowIsZero | deferred | BLS seal, governance, frozen-account lifecycle |
| TestUpdatePaymentParams | deferred | governance |

### permission_test.go (21)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestDeleteBucketPermission | ported | `TestPermissionAccountGrantEvmFlow` |
| TestDeletePolicy | ported | `TestStorageDeletePolicyEvmFlow` |
| TestCreateObjectByOthers | ported | `TestPermissionCreateObjectExpirationEvmFlow`: grantee create, owner and creator fields; the third-party cancelCreateObject refusal is portable |
| TestCreateObjectByOthersExpiration | ported | `TestPermissionCreateObjectExpirationEvmFlow` |
| TestCreateObjectByOthersLimitSize | ported | `TestPermissionCreateObjectLimitSizeEvmFlow` |
| TestGrantsPermissionToGroup | ported | `TestPermissionStaleGroupDenialEvmFlow` |
| TestVisibilityPermission | partial | bucket-level matrix in `TestStorageBucketVisibilityEvmFlow`; object visibility inheritance is portable |
| TestEmptyPermission | partial | as above |
| TestStalePermissionForAccountGC | deferred | portable: policy sweep after bucket deletion |
| TestDeleteObjectPolicy | partial | bucket-level revocation in `TestStorageDeletePolicyEvmFlow`; object-level policy is portable |
| TestDeleteGroupPolicy | partial | group deletion revokes in `TestPermissionStaleGroupDenialEvmFlow`; explicit deletePolicy for a group principal is portable |
| TestStalePermissionForGroupGC | deferred | portable: policy sweep after group deletion |
| TestGroupMembersAndPolicyGC | deferred | portable: member and policy sweep |
| TestExceedEachBlockLimitGC | deferred | portable: sweep batching across blocks |
| TestUpdateGroupExtraWithPermission | partial | owner path in `TestStorageUpdateGroupExtraEvmFlow`; grantee path is portable |
| TestPutPolicy_ObjectWithSlash | deferred | portable |
| TestVerifyStaleGroupPermission | ported | `TestPermissionStaleGroupDenialEvmFlow` |
| TestGrantsPermissionToObjectWithWildcardInName | deferred | portable |
| TestExpiredAccountPolicyGCAndRePut | partial | expiry enforcement and explicit delete of the expired policy in `TestPermissionCreateObjectExpirationEvmFlow`; sweep and re-put are portable |
| TestExpiredGroupPolicyGCAndRePut | partial | as above, group variant portable |
| TestSetResourceTagWithPermission | partial | owner setTag in `TestStorageSetTagEvmFlow`; grantee path is portable |

### sp_test.go (6)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestCreateStorageProvider | deferred | governance: SP creation is a proposal |
| TestEditStorageProvider | deferred | no precompile: edit-SP carries a BLS proof of possession |
| TestDeposit | deferred | no precompile for `x/sp` MsgDeposit |
| TestUpdateSpStoragePrice | ported | `TestStorageProviderPriceEvmFlow`: per-SP quote, global medians, secondary ratio, aggregation timing, by-time history; the UpdatePriceDisallowedDays refusal deferred: governance |
| TestUpdateStorageProviderParams | deferred | governance |
| TestUpdateStorageProviderStatus | deferred | no precompile for the maintenance status message |

### storage_bill_test.go (21)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestStorageBill_DeleteBucket_WithReadQuota | ported | `TestStorageBillDeleteBucketWithReadQuotaEvmFlow` |
| TestStorageBill_Smoke | deferred | BLS seal |
| TestStorageBill_DeleteObjectBucket_WithoutPriceChange | deferred | BLS seal |
| TestStorageBill_DeleteObjectBucket_WithPriceChange | deferred | BLS seal |
| TestStorageBill_DeleteObjectBucket_WithPriceChangeReserveTimeChange | deferred | BLS seal, governance |
| TestStorageBill_DeleteObject_WithStoreLessThanReserveTime | deferred | BLS seal |
| TestStorageBill_DeleteObject_WithStoreMoreThanReserveTime | deferred | BLS seal, governance |
| TestStorageBill_CreateBucket_WithZeroNoneZeroReadQuota | partial | charged-quota rate in `TestStorageBillDeleteBucketWithReadQuotaEvmFlow`; the zero-quota contrast is portable |
| TestStorageBill_CreateObject_WithZeroNoneZeroPayload | partial | lock fee on create in `TestStorageBillCancelCreateObjectEvmFlow`; the zero-payload contrast is portable |
| TestStorageBill_CreateObject_WithReserveTimeValidatorTaxRateChange | deferred | governance |
| TestStorageBill_CancelCreateObject | ported | `TestStorageBillCancelCreateObjectEvmFlow` |
| TestStorageBill_SealObject_WithoutPriceChange | deferred | BLS seal |
| TestStorageBill_SealObject_WithPriceChange | deferred | BLS seal |
| TestStorageBill_SealObject_WithPriceChangeValidatorTaxRateChange | deferred | BLS seal, governance |
| TestStorageBill_RejectSealObject_WithPriceChange | deferred | portable: rejectSealObject refund accounting needs no seal |
| TestStorageBill_FullLifecycle | deferred | BLS seal, governance |
| TestStorageBill_CopyObject_WithoutPriceChange | partial | copy path in `TestStorageCopyObjectEvmFlow`; billing of a stored source deferred: BLS seal |
| TestStorageBill_CopyObject_WithPriceChange | partial | as above |
| TestStorageBill_UpdateBucketQuota | deferred | portable: updateBucketInfo quota rate changes |
| TestStorageBill_UpdatePaymentAddress | deferred | BLS seal, migration |
| TestStorageBill_UpdateObject | partial | update and cancel path in `TestStorageCancelUpdateObjectContentEvmFlow`; billing of a completed update deferred: BLS seal |

### storage_rate_limit_test.go (7)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestSetBucketRateLimitToZero | ported | `TestStorageRateLimitEvmFlow` |
| TestSetBucketRateLimitToZeroAndDelete | deferred | BLS seal |
| TestNotOwnerSetBucketRateLimit_Object | deferred | portable: payment-account-owner rate limit matrix |
| TestNotOwnerSetBucketRateLimit_Bucket | deferred | portable: as above |
| TestNotOwnerSetBucketRateLimit_BucketPaymentAccount | deferred | portable: as above |
| TestQueryBucketRateLimit | deferred | portable: payment-account rate limit query |
| TestSetBucketFlowRateLimit_Discontinue | deferred | BLS seal |

### storage_test.go (24)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestCreateBucket | ported | `TestStorageEvmFlow` and the `createTestBucket` fixture |
| TestCreateObject | partial | create in `TestStorageBillCancelCreateObjectEvmFlow` and the permission tests; seal deferred: BLS seal |
| TestCreateGroup | ported | `TestPermissionStaleGroupDenialEvmFlow` |
| TestLeaveGroup | ported | `TestStorageLeaveGroupEvmFlow` |
| TestDeleteBucket | ported | `TestPermissionAccountGrantEvmFlow`, `TestStorageBillDeleteBucketWithReadQuotaEvmFlow` |
| TestDiscontinueObject_Normal | ported | `TestStorageDiscontinueObjectEvmFlow` |
| TestDiscontinueObject_UserDeleted | deferred | portable: delete refused on a discontinued object |
| TestDiscontinueBucket_Normal | ported | `TestStorageDiscontinueBucketEvmFlow` |
| TestDiscontinueBucket_UserDeleted | deferred | BLS seal |
| TestCancelCreateObject | ported | `TestStorageBillCancelCreateObjectEvmFlow` |
| TestCreateObjectWithCommonPrefix | deferred | BLS seal |
| TestUpdateParams | deferred | governance |
| TestCreateAndUpdateGroupExtraField | ported | `TestStorageUpdateGroupExtraEvmFlow` |
| TestCreateAndRenewGroup | ported | `TestStorageRenewGroupMemberEvmFlow` |
| TestRejectSealObject | deferred | portable: rejectSealObject on a created object |
| TestUpdateStorageParams | deferred | governance |
| TestMaintenanceSPCreateBucketAndObject | deferred | no precompile for the maintenance status message, BLS seal |
| TestCreateBucketAndSetTag | ported | `TestStorageSetTagEvmFlow` |
| TestCreateObjectAndSetTag | partial | bucket tag in `TestStorageSetTagEvmFlow`; object tag is portable, seal deferred: BLS seal |
| TestCreateGroupAndSetTag | deferred | portable: group tag and duplicate-group refusal |
| TestDeleteCreateObject_InCreatedStatus | partial | deleting an empty, auto-sealed object in `TestStorageDeleteObjectEvmFlow`; deleting a non-empty CREATED object is portable |
| TestDisallowChangePaymentAccount | deferred | BLS seal |
| TestToggleBucketSpAsDelegatedAgents | ported | `TestStorageToggleSPDelegatedAgentEvmFlow` |
| TestCreateObjectByDelegatedAgents | ported | `TestStorageDelegateCreateObjectEvmFlow`; seal deferred: BLS seal |

### virtualgroup_test.go (8)

| Retired test | Status | Covered by / reason |
| --- | --- | --- |
| TestBasic | partial | deposit in `TestVirtualGroupDepositEvmFlow`, create and delete in `setupPrimarySP` and `TestVirtualGroupDeleteGvgEvmFlow`; no precompile for GVG withdraw |
| TestSettle | deferred | BLS seal, SP exit/swap |
| TestUpdateVirtualGroupParams | deferred | governance |
| TestEmptyGlobalVirtualGroupFamily | partial | `TestVirtualGroupDeleteGvgEvmFlow` |
| TestSPExit | deferred | SP exit/swap |
| TestSPExit2 | deferred | SP exit/swap |
| TestSPForcedExit | deferred | SP exit/swap, governance |
| TestSPExit_SwapInfo_Expired | deferred | SP exit/swap, governance |
