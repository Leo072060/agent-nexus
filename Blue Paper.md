# 基于 ERC-8004 的 Validator 仲裁式数字交付 Escrow

## 摘要

本文提出一个面向数字商品与数字服务交付的交易协议层。它参考 ERC-8004 的 agent 身份、发现、声誉与验证思想，让卖家声明自己支持的 validator，让买家根据自己信任的 validator 筛选卖家，并通过 escrow 合约完成资金托管、交付证据固定和争议裁决。

当前 v1 合约采用独立 registry 实现，包含 `SellerRegistry`、`ValidatorRegistry` 和 `OrderRegistry`。ERC-8004 在本项目中作为长期集成方向：未来可以把卖家和 validator 的地址映射到 ERC-8004 agent 身份，并把交易反馈、裁决记录写入 Reputation Registry 或 Validation Registry。

协议的核心不是让智能合约自动判断交付质量，而是把交易过程中的关键事实结构化：卖家承诺卖什么、买家本次请求是什么、卖家实际交付了什么、validator 为什么作出某个裁决。链上保存 hash、状态和资金流，链下保存具体内容、证据和裁决报告。

## 背景与问题

数字商品和数字服务交易中，买卖双方通常面临四类问题。

首先，卖家身份和信誉难以组合。一个卖家可能在多个平台或 agent 市场中提供服务，但平台内身份、评分和交易记录很难迁移。

其次，买家不能选择自己的信任模型。传统平台通常由平台指定仲裁规则和风控方，但在开放 agent 经济中，不同买家可能信任不同的 validator，例如专业审计方、社区仲裁方、TEE 验证方、模型评测方或行业认证方。

第三，交付证据不够结构化。卖家可能声称已经交付，买家可能声称没有收到或质量不符。如果没有稳定的 hash 承诺，争议处理很容易退化成聊天截图和主观判断。

第四，资金托管与裁决权需要分离。直接付款给卖家会削弱买家保护，直接交给第三方又引入托管风险。更合适的方式是资金由智能合约托管，validator 只在争议发生时提交裁决，不能直接挪用资金。

## 参与者

### 卖家

卖家通过 `SellerRegistry` 使用自己的钱包地址注册。当前 v1 中，卖家地址就是卖家主键。

卖家注册时维护以下信息：

- `sellerURI`：卖家的公开资料 URI。
- `price`：当前唯一商品或服务的价格。
- `contentURI`：商品描述、服务说明、交付标准或预览信息。
- `contentHash`：`contentURI` 对应内容的 hash。
- `deliveryTimeout`：订单正式成立后，卖家承诺的默认交付时限。
- supported validators：卖家愿意接受裁决的 validator 地址列表。

第一版假设每个卖家只有一个商品或服务。后续如果支持多商品，可以把商品拆成独立 listing 或增加 listing registry。

### 买家

买家是创建订单并锁定资金的一方。买家在创建订单时选择卖家和 validator，并提交本次请求的 `requestHash`。

`requestHash` 对应买家的具体问题、需求、输入文件或服务说明。原始请求保存在链下，链上只保存 hash。争议发生时，validator 可以根据链下请求内容和链上 `requestHash` 验证争议对象是否一致。

### Validator

validator 通过 `ValidatorRegistry` 使用自己的钱包地址注册。当前 v1 中，validator 地址就是 validator 主键。

validator 维护以下信息：

- `validatorURI`：公开资料、联系方式、裁决规则和服务范围。
- `fee`：固定仲裁费用，使用 native ETH 计价。
- `responseTimeout`：争议开启后必须作出裁决的默认响应时限。
- `active`：是否当前接受新订单。

validator 不托管资金。资金始终在 `OrderRegistry` 中。validator 的权力仅限于争议状态下提交裁决。

### OrderRegistry

`OrderRegistry` 是当前 v1 的 escrow 合约，负责：

- 创建订单并锁定 `price + validatorFee`。
- 快照卖家的 `contentHash` 作为 `listingHash`。
- 保存买家提交的 `requestHash`。
- 等待卖家和 validator 依次确认订单。
- 记录卖家的 `deliveryHash`。
- 处理买家验收、交付超时退款、确认超时退款和争议裁决。

## 卖家支持的 Validator

本协议的第一个关键设计是让卖家声明自己支持哪些 validator。

卖家支持 validator 的含义是：如果交易发生争议，卖家同意接受这些 validator 的裁决，并让 escrow 合约根据裁决结果释放资金。买家不需要信任卖家支持的所有 validator，只需要找到自己信任集合与卖家支持集合之间的交集。

交易前的发现过程可以表达为：

```text
sellerSupportedValidators ∩ buyerTrustedValidators != ∅
```

如果交集为空，买家可以选择不交易，或要求卖家支持新的 validator。如果交集不为空，买家可以选择其中一个 validator 作为该订单的裁决方。

这个机制把“平台替用户决定谁可信”转变为“用户带着自己的信任偏好发现可交易对象”。在开放 agent 经济中，信任不是全局统一的，而是用户、场景、金额和风险共同决定的。

## 交易流程

### 1. 卖家与 Validator 注册

卖家先在 `SellerRegistry` 注册自己的资料、商品价格、商品说明 hash、默认交付时限，并添加自己愿意接受的 validator。

validator 在 `ValidatorRegistry` 注册自己的资料、仲裁费用和响应时限。

前端或 CLI 可以读取卖家列表、validator 列表和支持关系，帮助买家筛选合适卖家。

### 2. 买家创建订单并锁定资金

买家选择 seller、validator，并为本次需求生成 `requestHash`。创建订单时，买家向 `OrderRegistry` 支付：

```text
seller.price + validator.fee
```

合约会检查：

- seller 已注册且 active。
- validator 已注册且 active。
- seller 支持该 validator。
- `requestHash` 非零。
- 支付金额等于卖家价格加 validator 费用。

订单创建后进入 `PendingSeller` 状态。此时资金已锁定，但订单还没有正式成立。

### 3. 卖家先确认，Validator 后确认

当前 v1 采用三方确认模型，顺序是：

```text
PendingSeller -> PendingValidator -> Created
```

买家创建订单后，卖家先确认是否接单。卖家确认后，订单进入 `PendingValidator`。

validator 再确认是否接受本订单的争议裁决角色。validator 确认后，订单正式进入 `Created`，并从这一刻开始计算卖家的交付截止时间：

```text
deliveryDeadline = block.timestamp + seller.deliveryTimeout
```

如果 seller 或 validator 没有在 `approvalDeadline` 前确认，任意地址都可以调用退款函数，把订单本金和 validator fee 全部退回买家，状态变为 `ApprovalExpiredRefunded`。

### 4. 卖家提交交付 Hash

订单进入 `Created` 后，卖家必须在 `deliveryDeadline` 前提交 `deliveryHash`。

`deliveryHash` 是卖家实际交付内容或交付证据的 hash。它可以对应一个文件、一份回答、一次服务输出、一组链下证据，或某个内容包的 manifest。合约不理解交付物本身，只保存 hash。

提交成功后，订单进入 `DeliveryCommitted`。

如果卖家超过 `deliveryDeadline` 仍未交付，买家可以选择直接退款，使订单进入 `DeliveryExpiredRefunded`。此时订单已经由 seller 和 validator 确认成立，因此本金退给买家，validator fee 支付给 validator。买家也可以选择发起 dispute，让 validator 判断是否存在特殊情况或链下交付证据。

### 5. 买家验收或发起争议

在 `DeliveryCommitted` 状态下，买家可以检查卖家的交付内容。

如果买家认可交付结果，可以调用 `acceptDelivery`。合约会把订单本金转给卖家，并把 validator fee 退给买家。订单进入 `Released`。

如果买家认为交付不符合请求或商品承诺，买家可以发起 dispute。卖家如果认为买家恶意不验收，也可以在 `DeliveryCommitted` 状态下发起 dispute。

### 6. Validator 裁决

订单进入 `Disputed` 后，validator 在链下审查证据。证据可以包括：

- 卖家的商品描述或服务承诺。
- 买家的具体请求内容。
- 卖家的实际交付内容。
- 双方沟通记录。
- 文件元数据或其他链下证明。

validator 应检查链下材料是否匹配链上 hash：

```text
hash(listingContent) == listingHash
hash(buyerRequest) == requestHash
hash(deliveryContent) == deliveryHash
```

作出裁决后，validator 调用 `resolveDispute`，提交：

- `releaseToSeller`：本金给卖家还是退给买家。
- `resolutionHash`：裁决理由、报告或证据摘要的 hash。

合约执行资金分配：本金给胜方，validator fee 给 validator。订单进入 `ResolvedToSeller` 或 `ResolvedToBuyer`。

## Hash 证据语义

当前 v1 的证据链由四个 hash 构成。

`listingHash` 是卖家商品或服务承诺的 hash。它来自卖家注册或更新商品时的 `contentHash`，在订单创建时被快照。即使卖家后续修改商品描述，已有订单的 `listingHash` 也不会改变。

`requestHash` 是买家本次请求的 hash。它代表这笔订单的具体需求，不等同于卖家通用商品描述。对于 AI agent 服务、审计服务、咨询服务等场景，`requestHash` 能帮助 validator 判断交付是否回答了买家真正的问题。

`deliveryHash` 是卖家实际交付内容或交付证据的 hash。它不能自动证明质量正确，但可以固定争议对象，避免争议过程中替换交付物。

`resolutionHash` 是 validator 裁决理由或报告的 hash。链上不保存长文本裁决书，但可以用 hash 锚定链下报告，使裁决结果可追溯。

这四个 hash 共同形成最小证据链：

```text
seller promise -> buyer request -> seller delivery -> validator resolution
listingHash   -> requestHash   -> deliveryHash   -> resolutionHash
```

## 状态机

当前 `OrderRegistry` 的状态如下：

```text
None
PendingSeller
PendingValidator
Created
DeliveryCommitted
Disputed
Released
ApprovalExpiredRefunded
DeliveryExpiredRefunded
ResolvedToSeller
ResolvedToBuyer
```

正常成立路径：

```text
PendingSeller -> PendingValidator -> Created
```

正常完成路径：

```text
Created -> DeliveryCommitted -> Released
```

确认超时退款：

```text
PendingSeller/PendingValidator -> ApprovalExpiredRefunded
```

交付超时退款：

```text
Created -> DeliveryExpiredRefunded
```

交付超时退款时，本金退给买家，validator fee 支付给 validator。

争议裁决：

```text
DeliveryCommitted -> Disputed -> ResolvedToSeller
DeliveryCommitted -> Disputed -> ResolvedToBuyer
```

卖家超时未交付时，买家也可以选择进入 dispute：

```text
Created + deliveryDeadline expired -> Disputed
```

## 与 ERC-8004 的结合

当前 v1 合约没有直接依赖 ERC-8004 registry，而是先用地址作为主键完成最小可运行交易闭环。这样可以降低黑客松第一版的实现复杂度。

后续可以逐步接入 ERC-8004：

- Identity Registry：把 seller address 和 validator address 映射到 ERC-8004 agentId，让身份更可发现、可迁移、可组合。
- Reputation Registry：交易完成后记录卖家交付质量、validator 裁决质量和买家体验反馈。
- Validation Registry：把 validator 的裁决结果作为 validation response 锚定，使争议结果成为可索引的信任信号。

在这个方向上，本项目可以被理解为 ERC-8004 之上的交易协议层：ERC-8004 提供身份和信任基础，本协议提供订单、托管、证据和裁决执行。

## Demo 场景

### 发现与筛选

买家在 CLI 或前端中维护自己信任的 validator 集合。系统读取 `SellerRegistry` 中的卖家列表和每个卖家支持的 validator，只展示与买家信任集合存在交集的卖家。

买家还可以查看卖家的 `sellerURI`、商品价格、`contentURI`、`contentHash` 和默认交付时限。

### 正常交易

买家选择一个支持可信 validator 的卖家，提交本次需求的 `requestHash`，并锁定 `price + validatorFee`。

卖家先确认接单，validator 再确认接受仲裁角色。订单正式成立后，卖家在交付时限内提交 `deliveryHash`。买家检查交付内容后确认收货，合约把本金转给卖家，并把 validator fee 退还给买家。

### 交付超时

买家创建订单后，卖家和 validator 都确认了订单，但卖家没有在 `deliveryDeadline` 前提交交付。

买家可以直接触发退款，订单进入 `DeliveryExpiredRefunded`。此时本金退给买家，validator fee 支付给 validator。如果买家认为存在链下交付争议，也可以发起 dispute，让 validator 审查证据。

### 质量争议

卖家提交了 `deliveryHash`，但买家认为交付内容不符合商品承诺或本次请求。

validator 根据 `listingHash`、`requestHash`、`deliveryHash` 和链下证据判断交付是否达标。裁决后，validator 提交 `resolutionHash`，合约自动把本金给卖家或退给买家，并支付 validator fee。

## 设计原则

第一，资金由合约托管，validator 不托管资金。validator 只提交裁决，合约负责执行资金流。

第二，链上记录 hash、状态和资金结果，链下保存原始内容和证据。这样可以降低隐私泄露和链上存储成本。

第三，卖家支持 validator 是交易前信任发现机制。买家可以根据自己的信任偏好筛选卖家，而不是被统一平台规则绑定。

第四，订单成立需要三方确认。买家先锁款，卖家确认接单，validator 确认接受裁决角色，然后才开始计算交付时限。

第五，当前 v1 不自动判断质量。质量判断由双方选择的 validator 基于证据完成。

## 第一版范围

第一版聚焦数字商品和数字服务交付，不处理实物物流。

第一版不做部分退款，只支持裁决本金给卖家或买家。

第一版不要求链上加密交付。加密、私密证据提交、多 validator 仲裁、validator 质押和 slashing 可以作为后续增强能力。

第一版 registry 使用地址作为主键。未来可以把地址迁移或映射到 ERC-8004 agentId。
