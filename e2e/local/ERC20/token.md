# ERC20 合约记录（OpenZeppelin 版）

## 1. 合约编写

- 文件：`src/MyToken.sol`
- 基于 OpenZeppelin ERC20，18 位小数，构造参数为初始发行量。

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract MyToken is ERC20 {
    constructor(uint256 initialSupply) ERC20("MyToken", "MTK") {
        _mint(msg.sender, initialSupply);
    }
}
```

## 2. 脚本说明

- 部署脚本：`script/deploy.sh`
  - 用法：`./script/deploy.sh <local|devnet|testnet>`
  - 环境变量：
    - `PRIVATE_KEY`（必需）
    - `INITIAL_SUPPLY_WEI`（默认 1e24）
    - `FOUNDRY_EVM_VERSION`（默认 paris）
    - `RPC_URL`（可覆盖预设）

- 校验脚本：`script/verify.sh`
  - 用法：`./script/verify.sh <devnet|testnet> <contract_address>`
  - 变量：`INITIAL_SUPPLY_WEI`（默认 1e24）、`FOUNDRY_EVM_VERSION`（默认 paris）

## 3. 本地测试（Foundry + anvil）

- 启动本地链
```bash
anvil
```

- 运行测试（在 `ERC20` 目录）
```bash
FOUNDRY_EVM_VERSION=paris forge test -vv
```
- 结果：2 个用例全部通过（`testTotalSupply`、`testTransfer`）。



## 4. 合约部署

### Devnet

```bash
cd <project-root>/solidity
export PRIVATE_KEY=0x<你的 devnet 私钥>
FOUNDRY_EVM_VERSION=paris ./script/deploy.sh devnet
```
输出：
```text
Contract address: 0x1651ED0CB4234E051AA73232B202897A7808a69E
Tx hash: 0xdaed2289026bf8ebcd0de6ca3d0e986c53c70390210dadac66e0a1177ac82de6
symbol: "MTK"
totalSupply: 1000000000000000000000000
```

### Testnet

```bash
cd <project-root>/solidity
export PRIVATE_KEY=0x<你的 testnet 私钥>
FOUNDRY_EVM_VERSION=paris ./script/deploy.sh testnet
```
输出：
```text
Contract address: 0x5FbDB2315678afecb367f032d93F642f64180aa3
Tx hash: 0xfac03e04a5e765c508e57564cb6a97ffe715c0b16daaa6f3daae538d63d5793d
symbol: "MTK"
totalSupply: 1000000000000000000000000
```

## 5. 合约校验

- Devnet（本次结果）
```bash
FOUNDRY_EVM_VERSION=paris ./script/verify.sh devnet 0x1651ED0CB4234E051AA73232B202897A7808a69E
```
结果：已提交/已校验（浏览链接）
- https://devnet-scan.mocachain.org/address/0x1651ed0cb4234e051aa73232b202897a7808a69e

- Testnet（本次结果）
```bash
FOUNDRY_EVM_VERSION=paris ./script/verify.sh testnet 0x5FbDB2315678afecb367f032d93F642f64180aa3
```
结果：已提交/已校验（浏览链接）
- https://testnet-scan.mocachain.org/address/0x5fbdb2315678afecb367f032d93f642f64180aa3



## 6. 验证与交互示例

```bash
# 读取符号与发行量
cast call <合约地址> "symbol()(string)" --rpc-url <RPC>
cast call <合约地址> "totalSupply()(uint256)" --rpc-url <RPC>

# 转账 1 token（18 位）
RECIPIENT=0x 你的接收地址
cast send <合约地址> "transfer(address,uint256)" $RECIPIENT 1000000000000000000 \
  --rpc-url <RPC> --private-key $PRIVATE_KEY
```

### Devnet 交互示例

```bash
RPC=https://devnet-rpc.mocachain.org
ADDR=0x1651ED0CB4234E051AA73232B202897A7808a69E
RECIPIENT=0x 你的接收地址

# 读取
cast call $ADDR "symbol()(string)" --rpc-url $RPC
cast call $ADDR "totalSupply()(uint256)" --rpc-url $RPC

# 转账示例（需要为 $PRIVATE_KEY 地址准备 devnet 测试币）
cast send $ADDR "transfer(address,uint256)" $RECIPIENT 1000000000000000000 \
  --rpc-url $RPC --private-key $PRIVATE_KEY
```

实际输出：
```text
symbol: "MTK"
decimals: 18
totalSupply: 1000000000000000000000000 [1e24]
transfer tx: 0xaa32241408c4d457024ae861876c07b688bd2d9c2e0e99ec7b1458edc591ca60
recipient balance: 1000000000000000000 [1e18]
```

### Testnet 交互示例

```bash
RPC=https://testnet-rpc.mocachain.org
ADDR=0x5FbDB2315678afecb367f032d93F642f64180aa3
RECIPIENT=0x 你的接收地址

# 读取
cast call $ADDR "symbol()(string)" --rpc-url $RPC
cast call $ADDR "totalSupply()(uint256)" --rpc-url $RPC

# 转账示例（需要为 $PRIVATE_KEY 地址准备 testnet 测试币）
cast send $ADDR "transfer(address,uint256)" $RECIPIENT 1000000000000000000 \
  --rpc-url $RPC --private-key $PRIVATE_KEY
```

实际输出：
```text
symbol: "MTK"
decimals: 18
totalSupply: 1000000000000000000000000 [1e24]
transfer tx: 0xb7ddc640fe3745859e0c71ddb88c6e04371952f0126ee2fc9b69cdd998d0ab3d
recipient balance: 1000000000000000000 [1e18]
```

## 7. 备注
- 使用 OpenZeppelin 实现，源码参考：`@openzeppelin/contracts/token/ERC20/ERC20.sol`
- 若需加入权限（Ownable）、mint/burn 或可升级（UUPS），可在此合约基础上扩展。

 
