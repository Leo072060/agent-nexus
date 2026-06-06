# 基于 ERC-8004 的 Validator 仲裁式数字交付 Escrow

## 摘要

本文提出一个面向数字商品与数字服务交付的交易协议层。它参考 ERC-8004 的 agent 身份、发现、声誉与验证思想，让卖家声明自己支持的 validator，让买家根据自己信任的 validator 筛选卖家，并通过 escrow 合约完成资金托管、交付证据固定和争议裁决。

当前 v1 合约采用单一 `Market` 合约入口实现。`Market` 内部按模块组织 seller registry、validator registry 和 order escrow 逻辑，但部署和调用时只有一个市场地址。ERC-8004 在本项目中作为长期集成方向：未来可以把卖家和 validator 的地址映射到 ERC-8004 agent 身份，并把交易反馈、裁决记录写入 Reputation Registry 或 Validation Registry。

协议的核心不是让智能合约自动判断交付质量，而是把交易过程中的关键事实结构化：卖家承诺卖什么、买家本次请求是什么、卖家实际交付了什么、validator 为什么作出某个裁决。链上保存 hash、状态和资金流，链下保存具体内容、证据和裁决报告。

## 背景与问题

数字商品和数字服务交易中，买卖双方通常面临四类问题。

首先，卖家身份和信誉难以组合。一个卖家可能在多个平台或 agent 市场中提供服务，但平台内身份、评分和交易记录很难迁移。

其次，买家不能选择自己的信任模型。传统平台通常由平台指定仲裁规则和风控方，但在开放 agent 经济中，不同买家可能信任不同的 validator，例如专业审计方、社区仲裁方、TEE 验证方、模型评测方或行业认证方。

第三，交付证据不够结构化。卖家可能声称已经交付，买家可能声称没有收到或质量不符。如果没有稳定的 hash 承诺，争议处理很容易退化成聊天截图和主观判断。

第四，资金托管与裁决权需要分离。直接付款给卖家会削弱买家保护，直接交给第三方又引入托管风险。更合适的方式是资金由智能合约托管，validator 只在争议发生时提交裁决，不能直接挪用资金。

## 参与者

### 卖家

卖家通过 `Market` 的 seller 模块使用自己的钱包地址注册。当前 v1 中，卖家地址就是卖家主键。

卖家注册时维护以下信息：

- `sellerURI`：卖家 agent/service 的 base URL，用于公开卖家资料，并作为固定交付接口的 URL 前缀。
- `price`：当前唯一商品或服务的价格。
- `contentURI`：商品描述、服务说明、交付标准或预览信息。
- `contentHash`：`contentURI` 对应内容的 hash。
- `deliveryTimeout`：订单正式成立后，卖家承诺的默认交付时限。
- supported validators：卖家愿意接受裁决的 validator 地址列表。

第一版假设每个卖家只有一个商品或服务。后续如果支持多商品，可以把商品拆成独立 listing 或增加 listing registry。

v1 中卖家必须在 `sellerURI` 对应的服务上实现固定交付接口：

```text
POST {sellerURI}/agent-nexus/delivery
```

买家 CLI 在链上看到卖家提交 `deliveryHash` 后，会通过该接口领取交付内容。v1 不把交付内容上链，也不默认使用加密交付；访问控制由 seller service 验证 buyer 钱包签名完成。

### 卖家商品内容 JSON

当前 v1 推荐 `contentURI` 返回一个简单 JSON，作为卖家的商品或服务承诺。链上 `contentHash` 是 `contentURI` HTTP response body 原始 bytes 的 Keccak256 hash：

```text
contentHash = keccak256(contentURI response body)
```

buyer agent 或 CLI 在发现卖家时，应先请求 `contentURI`，校验 response body 的 hash 是否等于链上 `contentHash`。只有 hash 校验通过后，才解析 JSON 内容并展示给买家。

推荐的 v1 JSON 结构如下：

```json
{
  "schema": "agent-nexus.seller-content.v1",
  "title": "Solidity Contract Review",
  "summary": "Review one Solidity contract and return a short report.",
  "category": "code-review",
  "imageURI": "https://example.com/cover.png",
  "imageHash": "0x...",
  "deliverable": "Markdown report",
  "requirements": [
    "Buyer must provide Solidity source code or repository URL.",
    "Buyer must describe the review scope."
  ],
  "deliveryStandard": [
    "Report must include summary.",
    "Report must include findings.",
    "Report must include recommendations."
  ]
}
```

字段含义：

- `schema`：格式标识，帮助 agent 识别这是 Agent Nexus 的卖家商品描述。
- `title`：商品或服务名字。
- `summary`：一句话说明卖家提供什么。
- `category`：分类，方便搜索和过滤。
- `imageURI`：商品或服务展示图片、预览图、样例截图或作品图。
- `imageHash`：`imageURI` HTTP response body 原始 bytes 的 Keccak256 hash。
- `deliverable`：卖家承诺交付什么。
- `requirements`：买家下单前需要提供什么。
- `deliveryStandard`：买家验收和 validator 裁决时参考的交付标准。

`imageURI` 和 `imageHash` 是可选字段，但如果出现，必须成对出现。buyer agent 或 CLI 应请求 `imageURI` 并校验：

```text
imageHash = keccak256(imageURI response body)
```

如果 `contentHash` 不匹配，说明商品 JSON 与链上承诺不一致；如果 `imageHash` 不匹配，说明展示图片可能被替换。只有商品 JSON 和图片 hash 都通过时，前端或 CLI 才应把该卖家内容视为完整验证。

### 买家

买家是创建订单并锁定资金的一方。买家在创建订单时选择卖家和 validator，并提交本次请求的 `requestHash`。

`requestHash` 对应买家的具体问题、需求、输入文件或服务说明。原始请求保存在链下，链上只保存 hash。争议发生时，validator 可以根据链下请求内容和链上 `requestHash` 验证争议对象是否一致。

### Validator

validator 通过 `Market` 的 validator 模块使用自己的钱包地址注册。当前 v1 中，validator 地址就是 validator 主键。

validator 维护以下信息：

- `validatorURI`：validator service 的 base URL，用于公开资料、联系方式、裁决规则和服务范围，并作为固定争议证据接口的 URL 前缀。
- `fee`：固定仲裁费用，使用 native ETH 计价。
- `responseTimeout`：争议开启后必须作出裁决的默认响应时限。
- `active`：是否当前接受新订单。

validator 不托管资金。资金始终在 `Market` 合约中。validator 的权力仅限于争议状态下提交裁决。

v1 中 validator 必须在 `validatorURI` 对应的服务上实现固定争议证据接口：

```text
POST {validatorURI}/agent-nexus/disputes
```

买家 CLI 在链上打开 dispute 后，会通过该接口把链下证据包提交给 validator service。争议文本、请求原文和交付原文不上链，只保存在本地 DB 和 validator service 中。

### Market

`Market` 是当前 v1 的唯一链上入口。它内部包含 seller registry、validator registry 和 order escrow 三个逻辑模块，负责：

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

卖家先在 `Market` 注册自己的资料、商品价格、商品说明 hash、默认交付时限，并添加自己愿意接受的 validator。

validator 在 `Market` 注册自己的资料、仲裁费用和响应时限。

前端或 CLI 只需要配置 `marketAddress`，然后从 `Market` 读取卖家列表、validator 列表和支持关系，帮助买家筛选合适卖家。

### 2. 买家创建订单并锁定资金

买家选择 seller、validator，并为本次需求生成 `requestHash`。创建订单时，买家向 `Market` 支付：

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

## Buyer CLI

buyer CLI 是 buyer agent 的本地工具层。它不托管资金，也不替代链上 `Market` 的状态，而是帮助 buyer agent 完成市场配置、卖家发现、validator 信息同步、商品内容验证、订单创建与证据保存。

### 命令设计

`agent-nexus discover` 负责扫描所有 active markets，同步 validators 基础信息，读取每个 seller 支持的 validators，验证 seller 商品 JSON 和图片 hash，并把验证通过的商品写入本地 `products` 表、把 validator 信息写入本地 `validators` 表。它还会根据本地 `trusted_validators` 计算每个 product 与 buyer 信任集合的交集。`discover` 不直接返回完整商品列表，而是返回本轮发现状态摘要：

```json
{
  "status": "ok",
  "discoveryRunId": "2026-06-06T12:00:00Z",
  "marketsScanned": 2,
  "marketsSucceeded": 2,
  "marketsFailed": 0,
  "sellersScanned": 12,
  "productsVerified": 5,
  "productsInserted": 2,
  "productsUpdated": 3,
  "productsSkipped": 7,
  "validatorsScanned": 8,
  "validatorsInserted": 3,
  "validatorsUpdated": 5,
  "errors": []
}
```

`agent-nexus products` 负责读取本地数据库。默认情况下，它只展示最新一轮 `discover` 中验证通过、且支持至少一个 trusted validator 的商品，并只返回 agent 选择商品时最需要的摘要字段：

```json
{
  "discoveryRunId": "2026-06-06T12:00:00Z",
  "products": [
    {
      "id": 42,
      "title": "Solidity Contract Review",
      "summary": "Review one Solidity contract and return a short report.",
      "category": "code-review",
      "trustedValidatorMatches": [
        "0x1234567890123456789012345678901234567890"
      ]
    }
  ]
}
```

如果 buyer agent 需要查看某个商品的完整信息，可以调用：

```text
agent-nexus products --id 42
```

详情输出应包含 market、seller、price、`contentURI`、`chainContentHash`、`contentBodyHash`、`imageURI`、`imageHash`、`deliveryTimeout`、`supportedValidators`、`trustedValidatorMatches`、`firstSeenAt`、`lastSeenAt` 和完整商品字段。

buyer agent 可以用 validators 命令维护自己信任的 validator 集合：

```text
agent-nexus validators trust --market local --validator 0x1234567890123456789012345678901234567890 --label "Audit DAO"
agent-nexus validators untrust --market local --validator 0x1234567890123456789012345678901234567890
agent-nexus validators trusted
agent-nexus validators list
```

其中 `validators trust` 和 `validators untrust` 修改本地 `trusted_validators` 表；`validators trusted` 列出 buyer agent 当前信任集合；`validators list` 列出最近 discover 同步到的 validators。

`agent-nexus orders create` 负责从本地 product 快照创建订单，并在风控通过后自动签名上链：

```text
agent-nexus orders create --product 42 --request "帮我审计这个 Solidity 合约：..."
```

创建时，CLI 会使用本地 `products` 快照读取 market、seller、price 和 supported validators，选择 trusted validator，校验 product 来自最新 discover，校验价格、validator fee、每日消费限额和自动签名 policy。校验通过后，CLI 计算 `requestHash`，调用链上：

```text
Market.createOrder(seller, validator, requestHash, approvalTimeout)
```

其中 `approvalTimeout` 来自本地 CLI 配置。命令默认等待 seller 和 validator 确认，直到链上订单进入 `Created` 后返回。

`orders create` 成功输出示例：

```json
{
  "status": "created",
  "orderId": 1,
  "chainOrderId": "12",
  "seller": "0x2222222222222222222222222222222222222222",
  "validator": "0x1234567890123456789012345678901234567890",
  "requestHash": "0x...",
  "price": "10000000000000000",
  "validatorFee": "1000000000000000",
  "totalPayment": "11000000000000000",
  "next": "agent-nexus orders watch --id 1"
}
```

`agent-nexus orders watch --id 1` 负责领取交付内容。它监听链上 `DeliveryCommitted(orderId, deliveryHash)`，然后使用 buyer 钱包对固定消息签名，并请求卖家的固定交付接口：

```text
POST {sellerURI}/agent-nexus/delivery
```

请求 body：

```json
{
  "marketAddress": "0x...",
  "orderId": "12",
  "signature": "0x..."
}
```

buyer 钱包签名消息固定为：

```text
Agent Nexus delivery request
marketAddress: <marketAddress>
orderId: <orderId>
```

seller service 收到请求后，应根据 `marketAddress` 和 `orderId` 读取链上订单，验证签名地址等于链上 buyer，订单 seller 是自己，订单状态是 `DeliveryCommitted`。验证通过后，seller service 返回交付内容的原始 response body。

CLI 收到 response body 后计算 Keccak256，必须等于链上 `deliveryHash`。校验通过后，CLI 保存到本地 `orders.delivery` 并更新订单状态。

`orders watch` 成功输出示例：

```json
{
  "status": "delivery_received",
  "orderId": 1,
  "chainOrderId": "12",
  "deliveryHash": "0x...",
  "deliveryHashVerified": true,
  "deliverySaved": true
}
```

`agent-nexus orders dispute` 负责发起争议。买家提供本次请求、卖家回答内容和争议原因；如果买家没有收到卖家回答，`delivery` 可以省略。

```text
agent-nexus orders dispute \
  --id 1 \
  --request "帮我审计这个 Solidity 合约。" \
  --delivery "卖家的回答内容。" \
  --reason "卖家没有指出明显的重入风险，交付不符合 deliveryStandard。"
```

如果买家没有收到交付内容，可以省略 `--delivery` ：

```text
agent-nexus orders dispute \
  --id 1 \
  --request "帮我审计这个 Solidity 合约。" \
  --reason "Seller did not deliver before deadline."
```

CLI 会读取本地 `orders.id` 和链上订单，校验订单属于 buyer，校验链上状态允许 dispute：`DeliveryCommitted`，或 `Created` 且已经超过 `deliveryDeadline`。随后 CLI 校验 request 的 Keccak256 hash 等于链上 `requestHash`；如果提供了 delivery，则校验 delivery 的 Keccak256 hash 等于链上 `deliveryHash`。

校验通过后，CLI 保存 `request_content`、`delivery`、`dispute` 到本地 `orders` 表，调用链上：

```text
Market.openDispute(chainOrderId)
```

链上只记录订单进入 `Disputed` 状态，不保存争议文本。CLI 保存 `open_dispute_tx_hash`，更新本地 `status = Disputed`，然后用 buyer 钱包签名证据包，并发送给 validator service：

```text
POST {validatorURI}/agent-nexus/disputes
```

证据包格式：

```json
{
  "marketAddress": "0x...",
  "orderId": "12",
  "request": "...",
  "delivery": "...",
  "dispute": "...",
  "signature": "0x..."
}
```

validator service 收到证据包后，应验证签名地址等于链上 buyer，链上订单 validator 是自己，订单状态为 `Disputed`，request hash 与链上 `requestHash` 一致。如果 delivery 非空，还应验证 delivery hash 与链上 `deliveryHash` 一致。

如果 evidence POST 失败，链上 dispute 可能已经打开。CLI 应记录错误，并允许后续重试证据提交。

`orders dispute` 成功输出示例：

```json
{
  "marketAddress": "0x...",
  "orderId": "12",
  "status": "dispute_opened",
}
```

### 数据库设计

buyer CLI 使用 SQLite 作为本地记忆层。链上 `Market` 仍然是资金、订单状态和裁决结果的权威来源；本地数据库不替代链上状态，而是保存 buyer agent 在运行过程中看到的、生成的、验证过的链下材料。

本地数据库的核心目标是保存“当时看到的内容”。例如，卖家后续可能修改 `contentURI` 返回内容，或者某个 URL 失效；只要 buyer CLI 在发现阶段保存了 product 快照，后续订单和争议仍然可以复现买家下单时看到的卖家承诺。

| Table | Purpose | Key Identity |
| --- | --- | --- |
| `markets` | buyer agent 关注的 `Market` 合约 | `name` |
| `products` | 验证过的卖家商品快照 | `market_address + seller_address + content_body_hash` |
| `validators` | 每个 market 中的 validator 基础信息快照 | `market_address + validator_address` |
| `trusted_validators` | buyer agent 在每个 market 中信任的 validator 集合 | `market_address + validator_address` |
| `orders` | 买家订单、请求、争议和裁决结果 | local order id / chain order id |

`markets` 保存 buyer agent 关注的市场配置。一个 buyer agent 可以同时激活多个 markets，`discover` 默认查询所有 active markets。

`markets` 字段设计：

| Field | Meaning |
| --- | --- |
| `id` | 本地 market ID。 |
| `name` | buyer agent 给市场配置的本地名称。 |
| `rpc_url` | 查询该 market 所使用的链 RPC URL。 |
| `market_address` | 链上 `Market` 合约地址。 |
| `active` | 是否被 `discover` 默认查询。 |
| `created_at` | 本地 market 记录创建时间。 |
| `updated_at` | 本地 market 记录最后更新时间。 |

`validators` 保存每个 market 中 validator 的链上基础信息快照。因为同一个 validator 地址可以在不同 `Market` 合约中配置不同 fee 和 URI，所以 validator 的本地身份不是单独的钱包地址，而是：

```text
market_address + validator_address
```

`discover` 每轮会对 active markets 调用 `Market.getValidators()`，再对每个 validator 调用 `Market.getValidator(address)`，保存或更新本地 `validators` 表。第一版只保存链上字段，不主动请求 `validator_uri` 指向的链下 metadata。validator 是否当前接受新订单不作为本地 DB 字段保存；创建订单前，CLI 应重新读取链上 `Market` 状态确认 validator 可用。

`validators` 字段设计：

| Field | Meaning |
| --- | --- |
| `id` | 本地 validator ID。 |
| `discovery_run_id` | 最近一次发现并同步该 validator 的 discover 轮次 ID。 |
| `market_address` | validator 所属的链上 `Market` 合约地址。 |
| `validator_address` | validator 钱包地址。 |
| `validator_uri` | 链上登记的 validator 资料 URI。 |
| `fee` | 链上登记的 validator fee。 |
| `response_timeout` | 链上登记的争议响应时限。 |
| `first_seen_at` | CLI 第一次发现该 validator 的时间。 |
| `last_seen_at` | CLI 最近一次同步该 validator 的时间。 |

`trusted_validators` 保存 buyer agent 在每个 market 中主动信任的 validator 集合。信任按 market 隔离：同一个 validator 地址如果出现在两个不同 `Market` 合约中，需要分别加入信任集合。

```text
market_address + validator_address
```

`trusted_validators` 字段设计：

| Field | Meaning |
| --- | --- |
| `id` | 本地 trusted validator ID。 |
| `market_address` | trusted validator 所属的链上 `Market` 合约地址。 |
| `validator_address` | buyer agent 信任的 validator 钱包地址。 |
| `created_at` | 本地 trusted validator 记录创建时间。 |
| `updated_at` | 本地 trusted validator 记录最后更新时间。 |

`products` 保存 `discover` 阶段验证通过的卖家商品快照。它不是普通商品缓存，而是某个 market 上某个 seller 的某一版商品承诺。它的唯一身份语义是：

```text
market_address + seller_address + content_body_hash
```

同一个卖家修改商品 JSON 后，会形成新的 `products` 记录。一个 market 可以有多个 products，一个 product 可以被多个 orders 引用。

`products` 字段设计：

| Field | Meaning |
| --- | --- |
| `id` | 本地 product ID。 |
| `discovery_run_id` | 最近一次发现并验证该 product 的 discover 轮次 ID。 |
| `market_address` | product 所属的链上 `Market` 合约地址。 |
| `seller_address` | 卖家钱包地址。 |
| `seller_uri` | 链上登记的卖家资料 URI。 |
| `price` | 链上登记的商品价格。 |
| `content_uri` | 链上登记的商品 JSON URI。 |
| `chain_content_hash` | 链上登记的 `contentHash`。 |
| `content_body_hash` | CLI 实际请求 `content_uri` 后计算出的 body hash。 |
| `content_body` | `content_uri` 返回的原始 JSON body。 |
| `title` | 商品标题。 |
| `summary` | 商品摘要。 |
| `category` | 商品分类。 |
| `deliverable` | 卖家承诺交付内容。 |
| `requirements_json` | 买家下单前要求，按 JSON 保存。 |
| `delivery_standard_json` | 交付标准，按 JSON 保存。 |
| `image_uri` | 商品展示图片 URI。 |
| `image_hash` | 商品展示图片 hash。 |
| `image_body` | 图片原始 bytes，SQLite 中使用 BLOB 保存。 |
| `supported_validators_json` | 卖家支持的 validator 地址数组，按 JSON 保存。 |
| `delivery_timeout` | 链上登记的默认交付时限。 |
| `first_seen_at` | CLI 第一次发现该 product 的时间。 |
| `last_seen_at` | CLI 最近一次验证该 product 的时间。 |

其中 `content_body_hash` 是 CLI 实际请求 `contentURI` 后，对 HTTP response body 原始 bytes 计算得到的 hash。`chain_content_hash` 是链上 `Market.getSeller(address)` 返回的 `contentHash`。只有两者匹配时，CLI 才把该商品保存为 verified product。图片 bytes 使用 SQLite BLOB 保存。

`discover` 会读取 seller 支持的 validators，并写入 `supported_validators_json`。随后 CLI 用同一 `market_address` 下的 `trusted_validators` 计算交集，写入 `trusted_validator_matches_json`。`agent-nexus products` 默认只显示 `trusted_validator_matches_json` 非空的 products；`agent-nexus products --all` 才显示最新一轮全部 verified products。

每次 `discover` 开始时，CLI 会生成一个 `discoveryRunId`，建议使用 UTC timestamp 字符串。该轮同步到的 validators 和验证通过的 products 都写入同一个 `discovery_run_id`。`agent-nexus products` 默认通过最新的 `discovery_run_id` 查询最新一轮发现结果。第一版不单独增加 `discovery_runs` 表。

`orders` 保存买家的订单全过程。买家请求、争议理由和裁决结果都属于订单生命周期，因此第一版不单独拆成 `requests`、`disputes` 或 `resolutions` 表。一个 order 固定引用一个 product 快照，从而证明买家下单时看到的是哪一版卖家承诺。

`orders.validator_address` 应对应同一个 `market_address` 下的本地 validator 快照。buyer agent 后续可以根据 `trusted_validators` 选择自己信任、且 fee 和 responseTimeout 可接受的 validator；创建订单前再读取链上 `Market` 状态确认 validator 仍然可用。

`orders` 字段设计：

| Field | Meaning |
| --- | --- |
| `id` | 本地订单 ID。 |
| `product_id` | 本地 `products` 表中的商品快照 ID。 |
| `chain_order_id` | 链上 `Market` 合约里的订单 ID。 |
| `buyer_address` | 买家钱包地址。 |
| `seller_address` | 卖家钱包地址。 |
| `validator_address` | 订单选择的 validator 地址。 |
| `request_content` | 买家本次请求原文。 |
| `amount` | 买家的实际总成本记录，包含订单支付金额以及 buyer 侧 gas 成本。 |
| `price` | 商品价格。 |
| `validator_fee` | validator 费用。 |
| `create_order_tx_hash` | 买家创建订单交易 hash。 |
| `seller_confirmed_tx_hash` | 卖家确认接单交易 hash。 |
| `validator_confirmed_tx_hash` | validator 确认仲裁交易 hash。 |
| `delivery` | 卖家交付内容原文或本地保存的交付材料文本。 |
| `accept_tx_hash` | 买家确认收货交易 hash。 |
| `dispute` | 买家争议理由原文。 |
| `open_dispute_tx_hash` | 打开争议交易 hash。 |
| `resolution` | validator 裁决理由或报告原文。 |
| `resolved_to` | 裁决结果，取值为 `buyer` 或 `seller`。 |
| `resolve_tx_hash` | validator 裁决交易 hash。 |
| `status` | 本地同步的订单状态。 |
| `created_at` | 本地订单记录创建时间。 |
| `updated_at` | 本地订单记录最后更新时间。 |

订单表保存 request、delivery、dispute 和 resolution 的原文内容。对应 hash 可以由 CLI 在需要时临时计算，或从链上订单状态同步，不作为本地 `orders` 的独立列保存。

交易 hash 字段用于索引订单生命周期中的关键链上交易。`create_order_tx_hash`、`accept_tx_hash` 和 `open_dispute_tx_hash` 通常由 buyer CLI 发起后记录；`seller_confirmed_tx_hash`、`validator_confirmed_tx_hash` 和 `resolve_tx_hash` 可以由 CLI 同步链上事件后保存。

## Seller Service 设计

Seller service 是卖家 agent 的链下执行程序。它使用 seller 钱包管理注册信息、接单确认和交付提交；同时通过 HTTP 接口向通过身份验证的 buyer 返回交付内容。seller 私钥由 seller service 本地管理，不由 buyer CLI 管理。

详细实现手册见 `docs/Seller Service Manual.md`。

Seller service 需要负责：

- 使用 seller 钱包在 `Market` 注册或更新 `sellerURI`、商品价格、`contentURI`、`contentHash`、`deliveryTimeout` 和 supported validators。
- 监听或查询 `PendingSeller` 订单，决定是否调用 `Market.confirmAsSeller(orderId)`。
- 在订单进入 `Created` 后，根据买家请求生成交付内容，计算 `deliveryHash`。
- 调用 `Market.commitDelivery(orderId, deliveryHash)`，把交付 hash 固定到链上。
- 实现固定交付接口 `POST {sellerURI}/agent-nexus/delivery`，验证 buyer 签名和链上订单状态后返回交付内容。

Seller delivery 接口请求 body：

```json
{
  "marketAddress": "0x...",
  "orderId": "12",
  "signature": "0x..."
}
```

Seller service 收到请求后，应读取链上订单并验证：

- 签名地址等于链上 `buyer`。
- 链上订单 `seller` 是自己。
- 链上订单状态是 `DeliveryCommitted`。
- 返回内容的 Keccak256 hash 等于链上 `deliveryHash`。

Seller service 返回的是交付内容的原始 response body。合约不保存交付原文，链上只保存 `deliveryHash`、订单状态和资金结果。

## Validator Service 设计

Validator service 是 validator agent 的链下裁决程序。它使用 validator 钱包管理注册信息、确认仲裁角色、接收争议证据，并在裁决完成后把 `resolutionHash` 和资金归属结果提交到链上。validator 私钥由 validator service 本地管理，不由 buyer CLI 管理。

Validator service 需要负责：

- 使用 validator 钱包在 `Market` 注册或更新 `validatorURI`、`fee`、`responseTimeout` 和 active 状态。
- 监听或查询 `PendingValidator` 订单，决定是否调用 `Market.confirmAsValidator(orderId)`。
- 实现固定证据接口 `POST {validatorURI}/agent-nexus/disputes`，接收 buyer CLI 的链下证据包。
- 验证 buyer 签名、链上订单 validator、`status == Disputed`、`requestHash` 和可选 `deliveryHash`。
- 审查 product 快照、buyer request、seller delivery 和 dispute reason。
- 生成裁决报告，计算 `resolutionHash`。
- 调用 `Market.resolveDispute(orderId, releaseToSeller, resolutionHash)`。

Validator dispute 接口请求 body：

```json
{
  "marketAddress": "0x...",
  "orderId": "12",
  "buyerAddress": "0x...",
  "request": "...",
  "delivery": "...",
  "dispute": "...",
  "signature": "0x..."
}
```

Validator service 收到证据包后，应读取链上订单并验证：

- 签名地址等于链上 `buyer`。
- 链上订单 `validator` 是自己。
- 链上订单状态是 `Disputed`。
- `keccak256(request bytes)` 等于链上 `requestHash`。
- 如果 `delivery` 非空，`keccak256(delivery bytes)` 等于链上 `deliveryHash`。

Validator service 的裁决报告原文保存在链下，链上只保存 `resolutionHash` 和裁决资金流。`releaseToSeller = true` 表示本金给卖家；`releaseToSeller = false` 表示本金退买家。v1 不做部分退款。

## 状态机

当前 `Market` 中订单模块的状态如下：

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

买家在 CLI 或前端中维护自己信任的 validator 集合，并配置多个 active markets。buyer agent 运行 `agent-nexus discover` 后，CLI 会读取每个 active market 的卖家列表、validator 支持关系和 validators 基础信息，只展示与买家信任集合存在交集且 validator 配置可接受的卖家。

CLI 会请求卖家的 `contentURI`，验证 `contentBodyHash` 是否等于链上 `contentHash`。如果商品 JSON 中包含 `imageURI` 和 `imageHash`，CLI 也会验证图片内容并保存图片 bytes。验证通过的 product 会进入本地 SQLite，返回结果包含 `productId`、`contentBodyHash` 和 `imageHashVerified`。

买家还可以查看卖家的 `sellerURI`、商品价格、商品标题、摘要、分类、交付标准、默认交付时限，以及通过 `imageHash` 校验过的展示图片。

### 正常交易

买家选择一个支持可信 validator 的卖家，提交本次需求的 `requestHash`，并锁定 `price + validatorFee`。

卖家先确认接单，validator 再确认接受仲裁角色。订单正式成立后，卖家在交付时限内提交 `deliveryHash`。

buyer CLI 通过 `agent-nexus orders watch --id <id>` 监听 `DeliveryCommitted`，然后向 `{sellerURI}/agent-nexus/delivery` 提交 buyer 钱包签名来领取交付内容。CLI 会验证返回内容的 hash 等于链上 `deliveryHash`。买家检查交付内容后确认收货，合约把本金转给卖家，并把 validator fee 退还给买家。

### 交付超时

买家创建订单后，卖家和 validator 都确认了订单，但卖家没有在 `deliveryDeadline` 前提交交付。

买家可以直接触发退款，订单进入 `DeliveryExpiredRefunded`。此时本金退给买家，validator fee 支付给 validator。如果买家认为存在链下交付争议，也可以发起 dispute，让 validator 审查证据。

### 质量争议

卖家提交了 `deliveryHash`，但买家认为交付内容不符合商品承诺或本次请求。

validator 根据 `listingHash`、`requestHash`、`deliveryHash` 和链下证据判断交付是否达标。裁决后，validator 提交 `resolutionHash`，合约自动把本金给卖家或退给买家，并支付 validator fee。

## 设计原则

第一，资金由合约托管，validator 不托管资金。validator 只提交裁决，合约负责执行资金流。

第二，链上记录 hash、状态和资金结果，链下保存原始内容和证据。这样可以降低隐私泄露和链上存储成本。

buyer CLI 会把发现阶段验证通过的卖家 product 保存为本地快照，包括 `contentURI` 返回的原始 body、`contentBodyHash`、结构化字段和可选图片 bytes。这样即使后续 URL 内容变化或下线，买家仍能复现下单时看到的卖家承诺。

第三，卖家支持 validator 是交易前信任发现机制。买家可以根据自己的信任偏好筛选卖家，而不是被统一平台规则绑定。

第四，订单成立需要三方确认。买家先锁款，卖家确认接单，validator 确认接受裁决角色，然后才开始计算交付时限。

第五，当前 v1 不自动判断质量。质量判断由双方选择的 validator 基于证据完成。

## 第一版范围

第一版聚焦数字商品和数字服务交付，不处理实物物流。

第一版不做部分退款，只支持裁决本金给卖家或买家。

第一版不要求链上加密交付。加密、私密证据提交、多 validator 仲裁、validator 质押和 slashing 可以作为后续增强能力。

第一版 `Market` 内部 registry 模块使用地址作为主键。未来可以把地址迁移或映射到 ERC-8004 agentId。
