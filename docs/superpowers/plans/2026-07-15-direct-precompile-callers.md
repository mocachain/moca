# Direct Precompile Caller Semantics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow smart contracts to call transaction precompiles while making
the immediate EVM caller the single acting identity.

**Architecture:** The existing `RunNativeAction` boundary already journals
Cosmos writes for nested EVM calls. Remove the EOA-only guard from every
transaction method, bind `contract.Caller()` once per method, and use that
address consistently for message actor fields and event topics. Module message
servers continue enforcing authorization.

**Tech Stack:** Go, go-ethereum EVM, Cosmos SDK message servers, cosmos/evm precompile runtime, testify suites.

## Global Constraints

- Preserve target and counterparty arguments such as validator, bucket owner,
  group owner, storage provider, recipient, and grantee.
- Do not add a runtime fallback or height flag; coordinated `x/upgrade` binary replacement is the activation boundary.
- Preserve `RunNativeAction`, balance reconciliation, read-only checks, gas accounting, and module authorization.
- Branch, commit, and PR title follow Conventional Commits.

---

### Task 1: Pin Direct-Caller Behavior

**Files:**

- Modify: `precompiles/storage/tx_evm_apply_test.go`
- Modify: `precompiles/bank/tx_test.go`

**Interfaces:**

- Consumes: existing `Contract.CreateGroup`, `Contract.Send`, and test app keepers.
- Produces: regression tests proving `contract.Caller()` owns and funds forwarded actions independently of `evm.Origin`.

- [ ] **Step 1: Change the storage forwarding test to expect the contract caller to create the group**

```go
func (s *CreateGroupTestSuite) TestCreateGroup_AllowsContractForwarding() {
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	groupName := "regression-group-fwd"
	contract := vm.NewContract(caller, storage.GetAddress(), uint256.NewInt(0), 60_000, nil)
	contract.Input = s.mustPackCreateGroupInput(groupName, "")
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})
	c := storage.NewPrecompiledContract(s.app.StorageKeeper, s.app.BankKeeper)
	_, err := c.CreateGroup(s.ctx, evm, contract, false)
	s.Require().NoError(err)
	group, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, sdk.AccAddress(caller.Bytes()), groupName)
	s.Require().True(found)
	s.Require().Equal(sdk.AccAddress(caller.Bytes()).String(), group.Owner)
}
```

- [ ] **Step 2: Add a bank forwarding test with separate origin, caller, and recipient balances**

```go
func (s *PrecompileTestSuite) TestBankSend_AllowsContractForwarding() {
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	receiver := common.HexToAddress("0x4444444444444444444444444444444444444444")
	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(caller.Bytes()), 100))
	contract := vm.NewContract(caller, bank.GetAddress(), uint256.NewInt(0), bank.SendGas, nil)
	contract.Input = s.mustPackBankSendInput(receiver, big.NewInt(40))
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})
	c := bank.NewPrecompiledContract(s.app.BankKeeper, s.app.PaymentKeeper)
	_, err := c.Send(s.ctx, evm, contract, false)
	s.Require().NoError(err)
	s.Require().Equal(math.NewInt(60), s.balance(sdk.AccAddress(caller.Bytes())))
	s.Require().Equal(math.NewInt(40), s.balance(sdk.AccAddress(receiver.Bytes())))
	s.Require().Equal(math.NewInt(1_000_000_000_000), s.balance(sdk.AccAddress(s.address.Bytes())))
}
```

- [ ] **Step 3: Run the focused tests and verify both fail with the EOA-only error**

Run: `go test ./precompiles/storage ./precompiles/bank -run 'Test(CreateGroup|Precompile)TestSuite'`

Expected: both new forwarding tests fail with `only allow EOA can call this method`.

### Task 2: Adopt Direct Caller Across Transaction Precompiles

**Files:**

- Modify: `precompiles/{authz,bank,distribution,gov,payment,slashing,staking,storage,storageprovider,virtualgroup}/tx.go`

**Interfaces:**

- Consumes: `vm.Contract.Caller()` and existing Cosmos message server authorization.
- Produces: transaction methods that accept nested contract calls and use one direct-caller value for actor fields and logs.

- [ ] **Step 1: In all 68 transaction methods, remove the EOA-only block**

```go
if evm.Origin != contract.Caller() {
	return nil, errors.New("only allow EOA can call this method")
}
```

- [ ] **Step 2: Bind and consistently use the direct caller in each affected method**

```go
caller := contract.Caller()

msg := &banktypes.MsgSend{
	FromAddress: caller.String(),
}
```

Replace actor and actor-topic uses of `contract.Caller()` with `caller`; preserve target and counterparty arguments.

- [ ] **Step 3: Remove now-unused `errors` imports and format all affected files**

Run: `gofmt -w precompiles/{authz,bank,distribution,gov,payment,slashing,staking,storage,storageprovider,virtualgroup}/tx.go`

- [ ] **Step 4: Run focused tests and verify they pass**

Run: `go test ./precompiles/storage ./precompiles/bank -run 'Test(CreateGroup|Precompile)TestSuite'`

Expected: both packages pass, including forwarding tests.

- [ ] **Step 5: Verify no transaction precompile reads transaction origin**

Run: `rg 'evm\.Origin|only allow EOA' precompiles --glob '*.go'`

Expected: no matches.

### Task 3: Document and Verify the Consensus Change

**Files:**

- Modify: `precompiles/README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**

- Consumes: the behavior established in Tasks 1 and 2.
- Produces: operator-facing upgrade notice and developer-facing caller contract.

- [ ] **Step 1: Replace the EOA-only README section with direct-caller semantics**

```markdown
## Caller semantics (direct caller)

Transaction methods use the immediate EVM caller as the Cosmos message actor.
Smart contracts may call transaction precompiles, while module message servers
continue to enforce authorization. Transaction origin is not an authorization
input.
```

- [ ] **Step 2: Add an Unreleased changelog entry**

```markdown
- (precompiles) #280 Adopt direct-caller semantics for transaction precompiles, enabling smart-contract composition with native modules.
```

- [ ] **Step 3: Run repository verification**

Run: `go test ./precompiles/...`

Run: `go test ./app/...`

Run: `make lint`

Run: `go build ./...`

Expected: every command exits successfully.

- [ ] **Step 4: Review the diff against the design and commit**

Run: `git diff --check && git status --short && git diff --stat`

Commit: `feat(evm): adopt direct precompile caller semantics`
