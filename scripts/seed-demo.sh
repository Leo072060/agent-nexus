#!/usr/bin/env bash
#
# seed-demo.sh — 给 Agent Nexus 造一份演示数据。
#
# 做了什么：
#   1) 启动（或复用）本地 anvil
#   2) 用编译产物 out/Market.sol/Market.json 部署 Market
#   3) 注册 2 个卖家 + 2 个验证者（其中 validator ME = 看板里的“我”）
#   4) 造 6 笔覆盖各种状态的订单：
#        #1 待卖家确认        (validator ME)
#        #2 已创建待交付      (validator OTHER)
#        #3 已放款(happy path)(validator ME)
#        #4 争议中(进行中)    (validator ME) —— 私有库里有“证据已接收”
#        #5 裁决给卖家        (validator ME) —— 私有库里有完整 LLM 裁决
#        #6 裁决给买家        (validator OTHER) —— 看板显示“私有过程不可见”
#   5) 直接把 #4/#5 的私有判决数据写进 validator-service 的 SQLite
#   6) 写好 frontend/.env，并打印下一步命令
#
# 用法：
#   bash scripts/seed-demo.sh           # 对全新 anvil 运行（推荐）
#
# 需要：foundry(anvil/cast)、jq、sqlite3、node。全部本地只读，私钥用 anvil 默认测试账户。

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RPC="http://127.0.0.1:8545"
ARTIFACT="$ROOT/out/Market.sol/Market.json"
DBFILE="$ROOT/validator-service/validator-service.db"
ANVIL_LOG="$ROOT/scripts/anvil.log"

# --- anvil 默认测试私钥（公开、固定）---
K0=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 # owner / deployer
K1=0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d # seller A
K2=0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a # seller B
K3=0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6 # validator ME
K4=0x47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a # validator OTHER
K5=0x8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba # buyer A
K6=0x92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e # buyer B

echo "==> Agent Nexus 演示数据生成"

command -v anvil >/dev/null || { echo "缺少 anvil（foundry）"; exit 1; }
command -v cast  >/dev/null || { echo "缺少 cast（foundry）"; exit 1; }
command -v jq    >/dev/null || { echo "缺少 jq"; exit 1; }
command -v sqlite3 >/dev/null || { echo "缺少 sqlite3"; exit 1; }
[ -f "$ARTIFACT" ] || { echo "找不到合约产物 $ARTIFACT（先在 contracts 里 forge build）"; exit 1; }

# 派生地址
SELLER_A=$(cast wallet address --private-key "$K1")
SELLER_B=$(cast wallet address --private-key "$K2")
VAL_ME=$(cast wallet address --private-key "$K3")
VAL_OTHER=$(cast wallet address --private-key "$K4")
BUYER_A=$(cast wallet address --private-key "$K5")
BUYER_B=$(cast wallet address --private-key "$K6")

# --- 1) anvil ---
echo "[1/6] 准备 anvil…"
if cast block-number --rpc-url "$RPC" >/dev/null 2>&1; then
  echo "    检测到 anvil 已在运行，将复用（如需干净数据请先 pkill -f 'anvil --silent'）"
else
  nohup anvil --silent --host 127.0.0.1 --port 8545 > "$ANVIL_LOG" 2>&1 &
  ready=0
  for _ in $(seq 1 50); do
    cast block-number --rpc-url "$RPC" >/dev/null 2>&1 && { ready=1; break; }
    sleep 0.2 2>/dev/null || true
  done
  [ "$ready" = 1 ] || { echo "anvil 启动失败，见 $ANVIL_LOG"; exit 1; }
  echo "    anvil 已启动（日志 $ANVIL_LOG）"
fi

# 金额（wei）与超时（秒，给足 1 年，演示期间不过期）
PRICE_A=100000000000000000   # 0.1 ETH
PRICE_B=50000000000000000    # 0.05 ETH
FEE_ME=10000000000000000     # 0.01 ETH
FEE_OTHER=8000000000000000   # 0.008 ETH
TIMEOUT=31536000

CHASH_A=$(cast keccak "Seller A · GPT 文案服务 · v1")
CHASH_B=$(cast keccak "Seller B · 图像数据标注 · v1")

snd()  { local key="$1"; shift; cast send --rpc-url "$RPC" --private-key "$key" "$@" --json >/dev/null; }
sndtx(){ local key="$1"; shift; cast send --rpc-url "$RPC" --private-key "$key" "$@" --json | jq -r .transactionHash; }

# --- 2) 部署 Market ---
echo "[2/6] 部署 Market…"
BYTECODE=$(jq -r '.bytecode.object' "$ARTIFACT")
CARGS=$(cast abi-encode "constructor(string)" "ipfs://agent-nexus-demo-market")
INIT="${BYTECODE}${CARGS#0x}"
MARKET=$(cast send --rpc-url "$RPC" --private-key "$K0" --create "$INIT" --json | jq -r .contractAddress)
echo "    Market = $MARKET"

# --- 3) 注册卖家 / 验证者 ---
echo "[3/6] 注册卖家与验证者…"
snd "$K1" "$MARKET" "registerSeller(string,uint256,string,bytes32,uint256)" \
  "https://seller-a.example/agent-nexus" "$PRICE_A" "ipfs://seller-a/product" "$CHASH_A" "$TIMEOUT"
snd "$K2" "$MARKET" "registerSeller(string,uint256,string,bytes32,uint256)" \
  "https://seller-b.example/agent-nexus" "$PRICE_B" "ipfs://seller-b/product" "$CHASH_B" "$TIMEOUT"
snd "$K3" "$MARKET" "registerValidator(string,uint256,uint256)" "http://localhost:8082" "$FEE_ME" "$TIMEOUT"
snd "$K4" "$MARKET" "registerValidator(string,uint256,uint256)" "https://validator-other.example" "$FEE_OTHER" "$TIMEOUT"
# 卖家声明支持两个验证者
snd "$K1" "$MARKET" "addSupportedValidator(address)" "$VAL_ME"
snd "$K1" "$MARKET" "addSupportedValidator(address)" "$VAL_OTHER"
snd "$K2" "$MARKET" "addSupportedValidator(address)" "$VAL_ME"
snd "$K2" "$MARKET" "addSupportedValidator(address)" "$VAL_OTHER"

create_order() { # create_order <buyerKey> <seller> <validator> <requestHash> <value>
  snd "$1" "$MARKET" "createOrder(address,address,bytes32,uint256)" "$2" "$3" "$4" "$TIMEOUT" --value "$5"
}

echo "[4/6] 造 6 笔订单…"

# 演示文本
REQ1="请为新款静音咖啡机写一段 300 字推广文案，突出静音与快速出杯。"
REQ2="写 5 条夏季新品社媒短文案，活泼一点。"
REQ3="标注 500 张猫狗图片的二分类标签（猫/狗）。"
REQ4="写一篇 300 字静音咖啡机文案，必须强调静音与快速，并给出卖点清单。"
DEL4="静音咖啡机，出杯快。约 120 字的简短文案，未展开静音卖点。"
DISP4="交付仅约 120 字，未达 300 字要求，也没有突出静音卖点，与约定不符。"
REQ5="把这份 500 字英文产品说明翻译成中文，术语保持一致。"
DEL5="（完整的 500 字中文翻译，术语前后一致，覆盖全部段落。）"
DISP5="买家称翻译有误，但未指出任何具体错误句子。"
REQ6="标注 300 张人脸图片的情绪分类（开心/中性/生气）。"
DEL6="（仅交付约 50 张，且分类明显混乱。）"

RH1=$(cast keccak "$REQ1"); RH2=$(cast keccak "$REQ2"); RH3=$(cast keccak "$REQ3")
RH4=$(cast keccak "$REQ4"); RH5=$(cast keccak "$REQ5"); RH6=$(cast keccak "$REQ6")
DH3=$(cast keccak "已完成 500 张图片标注，自检准确率 99%。")
DH4=$(cast keccak "$DEL4"); DH5=$(cast keccak "$DEL5"); DH6=$(cast keccak "$DEL6")
DISPH4=$(cast keccak "$DISP4"); DISPH5=$(cast keccak "$DISP5")

# #1 PendingSeller — buyer A, seller A, validator ME
create_order "$K5" "$SELLER_A" "$VAL_ME" "$RH1" "$((PRICE_A+FEE_ME))"

# #2 Created — buyer A, seller A, validator OTHER
create_order "$K5" "$SELLER_A" "$VAL_OTHER" "$RH2" "$((PRICE_A+FEE_OTHER))"
snd "$K1" "$MARKET" "confirmAsSeller(uint256)" 2
snd "$K4" "$MARKET" "confirmAsValidator(uint256)" 2

# #3 Released — buyer B, seller B, validator ME
create_order "$K6" "$SELLER_B" "$VAL_ME" "$RH3" "$((PRICE_B+FEE_ME))"
snd "$K2" "$MARKET" "confirmAsSeller(uint256)" 3
snd "$K3" "$MARKET" "confirmAsValidator(uint256)" 3
snd "$K2" "$MARKET" "commitDelivery(uint256,bytes32)" 3 "$DH3"
snd "$K6" "$MARKET" "acceptDelivery(uint256)" 3

# #4 Disputed (进行中) — buyer B, seller A, validator ME
create_order "$K6" "$SELLER_A" "$VAL_ME" "$RH4" "$((PRICE_A+FEE_ME))"
snd "$K1" "$MARKET" "confirmAsSeller(uint256)" 4
snd "$K3" "$MARKET" "confirmAsValidator(uint256)" 4
snd "$K1" "$MARKET" "commitDelivery(uint256,bytes32)" 4 "$DH4"
snd "$K6" "$MARKET" "openDispute(uint256)" 4

# #5 ResolvedToSeller — buyer A, seller A, validator ME
create_order "$K5" "$SELLER_A" "$VAL_ME" "$RH5" "$((PRICE_A+FEE_ME))"
snd "$K1" "$MARKET" "confirmAsSeller(uint256)" 5
snd "$K3" "$MARKET" "confirmAsValidator(uint256)" 5
snd "$K1" "$MARKET" "commitDelivery(uint256,bytes32)" 5 "$DH5"
snd "$K5" "$MARKET" "openDispute(uint256)" 5
DECISION5='{"releaseToSeller":true,"summary":"交付完整且符合请求，判给卖家。","reasoning":"译文覆盖全部 500 字，术语前后一致；买家未能指出任何具体错误句子，主张缺乏依据。","buyerClaim":"声称翻译有误但无具体例证","sellerDeliveryAssessment":"完整、术语一致、符合请求","confidence":"high"}'
RES5=$(cast keccak "$DECISION5")
TX5=$(sndtx "$K3" "$MARKET" "resolveDispute(uint256,bool,bytes32)" 5 true "$RES5")

# #6 ResolvedToBuyer — buyer B, seller B, validator OTHER（非“我”，看板不展示私有过程）
create_order "$K6" "$SELLER_B" "$VAL_OTHER" "$RH6" "$((PRICE_B+FEE_OTHER))"
snd "$K2" "$MARKET" "confirmAsSeller(uint256)" 6
snd "$K4" "$MARKET" "confirmAsValidator(uint256)" 6
snd "$K2" "$MARKET" "commitDelivery(uint256,bytes32)" 6 "$DH6"
snd "$K6" "$MARKET" "openDispute(uint256)" 6
RES6=$(cast keccak "ruling-order-6-refund-buyer")
snd "$K4" "$MARKET" "resolveDispute(uint256,bool,bytes32)" 6 false "$RES6"

# --- 5) 私有判决数据写入 validator-service.db（仅 #4 #5，因其 validator = ME）---
echo "[5/6] 写入 validator-service 私有库（#4 进行中, #5 已裁决）…"
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
rm -f "$DBFILE"
sqlite3 "$DBFILE" <<SQL
CREATE TABLE IF NOT EXISTS disputes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	chain_order_id TEXT NOT NULL UNIQUE,
	buyer_address TEXT NOT NULL,
	seller_address TEXT NOT NULL,
	validator_address TEXT NOT NULL,
	request_hash TEXT NOT NULL,
	request_body BLOB NOT NULL,
	delivery_hash TEXT NOT NULL,
	delivery_body BLOB,
	dispute_hash TEXT NOT NULL,
	dispute_body BLOB NOT NULL,
	resolution_hash TEXT NOT NULL DEFAULT '',
	resolution_body BLOB,
	release_to_seller INTEGER NOT NULL DEFAULT 0,
	resolve_tx_hash TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
INSERT INTO disputes
 (chain_order_id, buyer_address, seller_address, validator_address, request_hash, request_body,
  delivery_hash, delivery_body, dispute_hash, dispute_body, status, created_at, updated_at)
VALUES
 ('4', '$BUYER_B', '$SELLER_A', '$VAL_ME', '$RH4', '$REQ4',
  '$DH4', '$DEL4', '$DISPH4', '$DISP4', 'evidence_received', '$NOW', '$NOW');
INSERT INTO disputes
 (chain_order_id, buyer_address, seller_address, validator_address, request_hash, request_body,
  delivery_hash, delivery_body, dispute_hash, dispute_body, resolution_hash, resolution_body,
  release_to_seller, resolve_tx_hash, status, created_at, updated_at)
VALUES
 ('5', '$BUYER_A', '$SELLER_A', '$VAL_ME', '$RH5', '$REQ5',
  '$DH5', '$DEL5', '$DISPH5', '$DISP5', '$RES5', '$DECISION5',
  1, '$TX5', 'resolved', '$NOW', '$NOW');
SQL

# --- 6) 写 frontend/.env ---
echo "[6/6] 写 frontend/.env…"
cat > "$ROOT/frontend/.env" <<ENV
VITE_RPC_URL=http://localhost:8545
VITE_MARKET_ADDRESS=$MARKET
VITE_VALIDATOR_API_BASE_URL=http://localhost:8082
VITE_DEFAULT_VIEW_AS=$VAL_ME
VITE_POLL_MS=8000
ENV

cat <<DONE

==================== 演示数据就绪 ====================
Market 合约 : $MARKET
RPC         : http://localhost:8545
“我”(validator ME) 地址 : $VAL_ME
“我”(validator ME) 私钥 : $K3

订单一览：
  #1 待卖家确认         验证者=我
  #2 已创建待交付       验证者=其他
  #3 已放款(happy path) 验证者=我
  #4 争议中(进行中)     验证者=我   → 私有库:证据已接收
  #5 裁决给卖家         验证者=我   → 私有库:完整 LLM 裁决(✓与链上一致)
  #6 裁决给买家         验证者=其他 → 看板显示“私有过程不可见”

下一步（开两个终端）：

  # ① 启动 validator-service（读私有判决数据）
  cd "$ROOT/validator-service" && \\
  VALIDATOR_RPC_URL=http://localhost:8545 \\
  VALIDATOR_MARKET_ADDRESS=$MARKET \\
  VALIDATOR_PRIVATE_KEY=$K3 \\
  VALIDATOR_BASE_URL=http://localhost:8082 \\
  DEEPSEEK_API_KEY=dummy-not-used-for-reads \\
  go run ./cmd/validator-service serve

  # ② 启动前端看板
  cd "$ROOT/frontend" && npm run dev   # http://localhost:5173

frontend/.env 已自动写好。anvil 在后台运行；停止：pkill -f 'anvil --silent'
=====================================================
DONE
