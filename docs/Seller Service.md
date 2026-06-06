# Seller Service

## 目标

Seller service 是卖家 agent 的链下执行程序。它负责管理 seller 钱包、注册和更新商品、确认订单、生成交付内容、提交 `deliveryHash`，并在 buyer 通过身份验证后返回交付内容。

Seller service 不托管 buyer 资金。资金始终由链上 `Market` 合约托管，seller service 只管理 seller 自己的钱包和交付流程。

## 核心职责

Seller service 第一版需要完成以下工作：

- 使用 seller 钱包在 `Market` 注册或更新卖家资料、商品配置和 supported validators。
- 监听或查询待 seller 确认的订单。
- 对可接受的订单调用 `Market.confirmAsSeller(orderId)`。
- 在订单正式进入 `Created` 后生成交付内容。
- 计算交付内容原始 bytes 的 Keccak256 hash，得到 `deliveryHash`。
- 调用 `Market.commitDelivery(orderId, deliveryHash)`，把交付 hash 固定到链上。
- 实现 `POST {sellerURI}/agent-nexus/delivery`，向通过 buyer 签名验证的请求返回交付内容。

## 配置

Seller service 至少需要以下配置：

| Config | Meaning |
| --- | --- |
| `rpc_url` | 读取和写入链上 `Market` 的 RPC URL。 |
| `market_address` | 链上 `Market` 合约地址。 |
| `seller_private_key` | seller 钱包私钥，只由 seller service 本地使用。 |
| `seller_uri` | seller service 的 base URL，对应链上 `sellerURI`。 |
| `content_uri` | 商品 JSON URI。 |
| `content_hash` | `content_uri` response body 的 Keccak256 hash。 |
| `price` | 商品价格。 |
| `delivery_timeout` | 默认交付时限。 |
| `supported_validators` | seller 愿意接受裁决的 validator 地址列表。 |

## 建议模块

Seller service 可以按以下模块组织：

| Module | Responsibility |
| --- | --- |
| `config` | 读取 RPC、market address、seller wallet、sellerURI 和商品配置。 |
| `chain client` | 读取 `Market` 状态并发送 seller 交易。 |
| `order watcher` | 发现 `PendingSeller` 和 `Created` 订单。 |
| `delivery engine` | 根据 buyer request 生成交付内容。 |
| `local store` | 保存订单、delivery body、deliveryHash 和交易 hash。 |
| `http server` | 提供 buyer 领取交付内容的 HTTP 接口。 |

第一版不强制规定数据库结构。只要 seller service 能在 buyer 请求交付时，通过 `orderId` 找回对应 delivery body 即可。

## 链上动作

### 注册或更新商品

Seller service 使用 seller 钱包调用 `Market` 的 seller 模块，完成卖家注册、商品更新和 supported validators 维护。

链上商品配置中的 `contentHash` 必须等于 `contentURI` HTTP response body 原始 bytes 的 Keccak256 hash。

### 确认订单

当订单状态为 `PendingSeller`，且 seller service 决定接单时，调用：

```text
Market.confirmAsSeller(orderId)
```

调用成功后，订单进入 `PendingValidator`，等待 validator 确认。

### 提交交付 Hash

当订单进入 `Created` 后，seller service 生成交付内容，并计算：

```text
deliveryHash = keccak256(delivery response body bytes)
```

然后调用：

```text
Market.commitDelivery(orderId, deliveryHash)
```

调用成功后，订单进入 `DeliveryCommitted`。从这一刻起，buyer CLI 可以通过 seller delivery 接口领取交付内容。

## Delivery 接口

Seller service 必须实现固定路径：

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

Seller service 收到请求后，应读取链上订单并验证：

- `marketAddress` 是当前 service 配置的 `Market` 地址。
- 签名地址等于链上订单的 `buyer`。
- 链上订单的 `seller` 等于当前 seller 钱包地址。
- 链上订单状态是 `DeliveryCommitted`。
- 本地存在该 `orderId` 对应的 delivery body。
- `keccak256(delivery body bytes)` 等于链上 `deliveryHash`。

验证通过后，seller service 返回 delivery body 原始 bytes。buyer CLI 会再次计算 response body 的 Keccak256，并要求它等于链上 `deliveryHash`。

## 状态流

Seller service 主要处理以下订单状态：

```text
PendingSeller -> PendingValidator -> Created -> DeliveryCommitted
```

- `PendingSeller`：seller service 判断是否接单。
- `PendingValidator`：seller 已确认，等待 validator 确认。
- `Created`：订单正式成立，seller service 应在 `deliveryDeadline` 前生成交付。
- `DeliveryCommitted`：seller 已提交 `deliveryHash`，等待 buyer 领取和验收。

如果 seller service 没有在 `deliveryDeadline` 前提交 `deliveryHash`，buyer 可以退款或发起 dispute。

## 错误处理

- 如果 `confirmAsSeller` 失败，应记录交易错误，并保留订单状态供后续重试或人工处理。
- 如果生成交付失败，应在本地记录失败原因，不应提交空的 `deliveryHash`。
- 如果 `commitDelivery` 失败，应保留 delivery body 和 `deliveryHash`，允许重试提交。
- 如果 delivery 接口签名验证失败，应拒绝请求，不返回交付内容。
- 如果本地 delivery body 的 hash 与链上 `deliveryHash` 不一致，应拒绝返回，并标记为严重一致性错误。

## 安全原则

- seller 私钥只由 seller service 本地使用，不暴露给 buyer CLI。
- 交付内容不上链，链上只保存 `deliveryHash`。
- delivery 接口只向链上订单 buyer 返回交付内容。
- HTTP 返回的原始 body 必须稳定，因为 buyer CLI 会直接对 response body bytes 计算 hash。
- seller service 不应在未提交 `deliveryHash` 前向 buyer 返回最终交付内容。

## 第一版范围

第一版 seller service 只定义进程职责和协议接口，不限定具体编程语言、Web 框架或数据库。后续实现时可以先用最小 HTTP server、SQLite local store 和 go-ethereum chain client 完成 demo。
