# Agent Nexus CLI Market Frontend

面向 CLI 用户的只读市场前端。它读取链上 Market 数据，展示卖家、验证者、订单和争议状态。完整诉讼详情需要订单买家、卖家或验证者使用钱包签名后查看。

## 运行

```bash
cd cli-frontend
cp .env.example .env
npm install
npm run dev
```

## 环境变量

| 变量 | 说明 |
| --- | --- |
| `VITE_RPC_URL` | 链 RPC 端点 |
| `VITE_MARKET_ADDRESS` | Market 合约地址 |
| `VITE_POLL_MS` | 可选，自动刷新间隔 ms，默认 8000 |

## 构建

```bash
npm run build
```
