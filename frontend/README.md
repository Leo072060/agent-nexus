# Agent Nexus · Validator 看板（前端）

只读看板，给 validator（检查者）运营方看：

- 某个 Market 里**所有交易**（买家 ↔ 卖家、验证者、金额、状态）；
- 哪些订单**有争议 / 已裁决**，以及链上裁决结果（放款给卖家 / 退款给买家）；
- 如果某笔订单的验证者**就是我**，展示完整的**私有判决流程**（证据原文 + LLM 的 summary/reasoning/confidence + resolveTx）。

## 数据来源

- **链上公开数据**：浏览器用 [viem](https://viem.sh) 直连 RPC 读 `Market` 合约（`getOrderCount` / `getOrder` / `marketURI` / `owner`）。
- **私有判决数据**：调用 `validator-service` 的只读接口
  - `GET /agent-nexus/me` —— 我是哪个验证者
  - `GET /agent-nexus/disputes` —— 我处理过的争议列表
  - `GET /agent-nexus/disputes/{orderId}` —— 单条裁决全过程

“这个验证者是不是我”：用 `/me` 返回的地址和每笔订单的 `order.validator` 比对；顶部还有“以某地址身份查看”输入框，演示时可切换视角。

## 技术栈

React 18 + Vite + TypeScript + viem。无需钱包（全程只读）。

## 运行

```bash
cd frontend
cp .env.example .env      # 填入 RPC / Market 地址 / validator-service 地址
npm install
npm run dev               # http://localhost:5173
```

### 环境变量（`.env`）

| 变量 | 说明 |
| --- | --- |
| `VITE_RPC_URL` | 链 RPC 端点（如 `http://localhost:8545`） |
| `VITE_MARKET_ADDRESS` | 当前 Market 合约地址 |
| `VITE_VALIDATOR_API_BASE_URL` | validator-service 地址，默认 `http://localhost:8082` |
| `VITE_DEFAULT_VIEW_AS` | 可选，默认“以某验证者身份查看”的地址 |
| `VITE_POLL_MS` | 可选，自动刷新间隔(ms)，0 关闭 |

> validator-service 需带 `DEEPSEEK_API_KEY`（只读演示可填任意 dummy 值，新 GET 接口不会调用 LLM），并已开启 CORS。

## 构建

```bash
npm run build        # tsc --noEmit + vite build
npm run preview      # 预览 dist
```

## ABI 来源

`src/abi/market.ts` 是从 `../out/Market.sol/Market.json` 拷贝生成的（`out/` 是 Foundry 产物，`forge clean` 会删，所以拷贝一份解耦）。合约变更后重新生成：

```bash
node -e "const fs=require('fs');const abi=require('./out/Market.sol/Market.json').abi;fs.writeFileSync('frontend/src/abi/market.ts','export const marketAbi = '+JSON.stringify(abi,null,2)+' as const;\n')"
```
（在仓库根目录执行。）
