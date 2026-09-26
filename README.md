# GeoIP 服務

這是一個簡單的 GeoIP 服務，使用 MaxMind GeoLite2 資料庫來查詢 IP 地址的地理位置資訊。

## 功能

- 查詢 IP 地址的國家、地區和城市
- 支援健康檢查端點
- 支援多種配置方式（CLI、環境變數、預設路徑）

## 目錄結構

```bash
mkdir -p /opt/geoip/{bin,data,conf,logs}
```

## 環境要求

- **Go 版本**: 1.24.0 或更高
- **系統支援**: Linux、macOS、Windows
- **資料庫**: MaxMind GeoLite2 或 DB-IP mmdb 格式資料庫
- **磁碟空間**: 至少 100MB（用於資料庫及日誌）

## 編譯

### 前置步驟

1. 確保已安裝 Go 1.24.0 或更高版本：
```bash
go version
```

2. 克隆或下載專案：
```bash
git clone https://github.com/wyusgw/GeoIP.git
cd GeoIP
```

3. 下載依賴：
```bash
go mod download
go mod tidy
```

### 編譯步驟

**Linux/macOS 本地編譯**:
```bash
go build -o geoip-service main.go
```

**交叉編譯為 Linux amd64**（適合在 macOS 或 Windows 上編譯給 Linux 伺服器使用）:
```bash
GOOS=linux GOARCH=amd64 go build -o geoip-service main.go
```

**其他架構**:
```bash
# 編譯為 Linux arm64（例如 Apple Silicon Mac 或 ARM 伺服器）
GOOS=linux GOARCH=arm64 go build -o geoip-service main.go

# 編譯為 Windows
GOOS=windows GOARCH=amd64 go build -o geoip-service.exe main.go
```

編譯完成後，你會得到可執行檔 `geoip-service`（Windows 上為 `geoip-service.exe`）。

### 驗證編譯

```bash
./geoip-service -h
# 若無 -h 參數，可嘗試執行，應看到「failed to open config」或類似錯誤（表示程式正常啟動但缺少配置）
```

## 安裝與配置

### 步驟 1: 準備目錄結構

```bash
# 建立服務目錄
mkdir -p /opt/geoip/{bin,data,conf,logs}

# 將編譯好的執行檔複製到 bin 目錄
cp geoip-service /opt/geoip/bin/

# （可選）建立符號連結以便在任何地方執行
sudo ln -sf /opt/geoip/bin/geoip-service /usr/local/bin/geoip-service
```

### 步驟 2: 取得資料庫

**選項 A: MaxMind GeoLite2（免費）**

1. 前往 [MaxMind GeoLite2](https://www.maxmind.com/en/geolite2/signup) 註冊帳號並登入
2. 下載 `GeoLite2-City.mmdb`
3. 將檔案複製到 `/opt/geoip/data/`

```bash
cp GeoLite2-City.mmdb /opt/geoip/data/
chmod 644 /opt/geoip/data/GeoLite2-City.mmdb
```

**選項 B: DB-IP（免費 Lite 版本）**

1. 前往 [DB-IP](https://db-ip.com/) 下載 `dbip-city-lite.mmdb`
2. 將檔案複製到 `/opt/geoip/data/`

```bash
cp dbip-city-lite.mmdb /opt/geoip/data/
chmod 644 /opt/geoip/data/dbip-city-lite.mmdb
```

### 步驟 3: 建立配置檔案

在 `/opt/geoip/conf/config.json` 建立配置檔案：

```bash
cat > /opt/geoip/conf/config.json << 'EOF'
{
  "port": 8080,
  "mmdb_path": "/opt/geoip/data/GeoLite2-City.mmdb",
  "enable_health": true,
  "name_mapping_path": "/opt/geoip/conf/mapping.json"
}
EOF
```

或如果使用 DB-IP：

```bash
cat > /opt/geoip/conf/config.json << 'EOF'
{
  "port": 8080,
  "mmdb_path": "/opt/geoip/data/dbip-city-lite.mmdb",
  "enable_health": true,
  "name_mapping_path": ""
}
EOF
```

**配置欄位說明**:

| 欄位 | 類型 | 必需 | 預設值 | 說明 |
|------|------|------|--------|------|
| `port` | 整數 | 否 | 8080 | 服務監聽的 TCP 連接埠 |
| `mmdb_path` | 字串 | 是 | 無 | MaxMind/DB-IP mmdb 資料庫檔案的絕對路徑 |
| `enable_health` | 布林 | 否 | false | 是否啟用 `/api/v1/health` 端點 |
| `name_mapping_path` | 字串 | 否 | 空字串 | 本地翻譯對照表 JSON 檔路徑（見下方「本地翻譯對照表」）；留空則不啟用 |

### 步驟 4: 配置路徑優先順序

服務會依以下順序查找配置檔案（先找到的使用）：

1. **CLI 參數** (最高優先):
   ```bash
   ./geoip-service -config /etc/myconfig.json
   ```

2. **環境變數**:
   ```bash
   export CONFIG=/etc/myconfig.json
   ./geoip-service
   ```

3. **預設路徑** (依順序查找):
   - `./config.json` (當前目錄)
   - `./config/config.json` (當前目錄下的 config 子目錄)
   - `/etc/geoip/config.json` (系統全域)

## 運行

### 本地開發環境

1. 在專案根目錄建立 `config.json`：
```bash
cat > config.json << 'EOF'
{
  "port": 8080,
  "mmdb_path": "./GeoLite2-City.mmdb",
  "enable_health": true,
  "name_mapping_path": ""
}
EOF
```

2. 將資料庫檔案放到專案根目錄

3. 直接執行：
```bash
./geoip-service
```

服務應在 `http://localhost:8080` 啟動。

### 生產環境（使用已安裝的目錄結構）

```bash
# 方法 1: 指定配置路徑
/opt/geoip/bin/geoip-service -config /opt/geoip/conf/config.json

# 方法 2: 使用環境變數
export CONFIG=/opt/geoip/conf/config.json
/opt/geoip/bin/geoip-service

# 方法 3: 使用符號連結且配置在預設路徑
geoip-service
```

### 搭配反向代理（nginx/Apache）

若服務前面有反向代理，請在 nginx 配置中設定以下 header（以便服務正確取得用戶真實 IP）：

**nginx 範例**:
```nginx
location /api/v1/geoip {
    proxy_pass http://localhost:8080;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

之後查詢時無需帶 `ip` 參數，服務會自動讀取 `X-Forwarded-For` 或 `X-Real-IP`。

### 背景執行與日誌

**使用 nohup 在背景執行**:
```bash
nohup /opt/geoip/bin/geoip-service -config /opt/geoip/conf/config.json > /opt/geoip/logs/geoip.log 2>&1 &
```

**檢視日誌**:
```bash
tail -f /opt/geoip/logs/geoip.log
```

## API 使用

所有端點均以 `/api/v1` 為前綴。

### 查詢單一 IP 地理位置

**端點**: `GET /api/v1/geoip`

**參數**:
- `ip` (可選): 要查詢的 IP 地址。如果未提供，則使用請求者的 IP。
- `lang` (可選): 回應語言，支援 `en`（預設）、`zh-CN`、`ja`、`ko`、`ru`、`fr`、`de`、`es`、`pt-BR`、`fa`。若資料庫中無對應翻譯（例如 MaxMind GeoLite2 未內建 `ko`、`fa` 譯名），會自動回退為 `en`。
- `fields` (可選): 以逗號分隔要回傳的欄位，可選 `ip`、`country`、`region`、`city`、`country_geoname_id`、`region_geoname_id`、`city_geoname_id`。未提供時回傳所有基本欄位（`ip`/`country`/`region`/`city`，不含 `*_geoname_id`）；帶入未知欄位會回傳 `400`。`*_geoname_id` 欄位回傳 mmdb 原始的 GeoNames geonameid，用來核對「本地翻譯對照表」章節裡對照表的 key 是否正確。
- `translate` (可選，布林值，預設 `false`): 是否在資料庫沒有該語言翻譯時，套用本地翻譯對照表（`name_mapping_path`）作為 fallback。資料庫本身若已有對應語言的翻譯，一律優先採用資料庫的值，對照表只補資料庫沒有的部分。

**回應**:
```json
{
  "country": "United States",
  "region": "California",
  "city": "San Francisco",
  "ip": "8.8.8.8"
}
```

**範例**:
```bash
curl "http://localhost:8080/api/v1/geoip?ip=8.8.8.8&lang=zh-CN"
```

不帶 `ip` 參數時，改查詢請求者自身的 IP（優先讀取 `X-Forwarded-For`、其次 `X-Real-IP`，最後才用 TCP 連線的來源位址；服務前面若有反向代理，需自行設定轉送這些 header）：
```bash
curl "http://localhost:8080/api/v1/geoip"
```

只回傳部分欄位：
```bash
curl "http://localhost:8080/api/v1/geoip?ip=8.8.8.8&fields=country,city"
```
```json
{
  "country": "United States",
  "city": "San Francisco"
}
```

指定語言與欄位過濾：
```bash
curl "http://localhost:8080/api/v1/geoip?ip=8.8.8.8&lang=zh-CN&fields=country,region,city"
```

套用本地翻譯對照表作為 fallback（需先在配置中設定 `name_mapping_path`）。當資料庫本身無對應語言的翻譯時，會從對照表中查找；若對照表也沒有，則回退為英文原名：
```bash
curl "http://localhost:8080/api/v1/geoip?ip=8.8.8.8&lang=zh-CN&translate=true"
```

回應範例（假設對照表有 China / Zhejiang 的中文翻譯，但 San Francisco 無翻譯）：
```json
{
  "country": "中国",
  "region": "浙江",
  "city": "San Francisco",
  "ip": "8.8.8.8"
}
```

**錯誤情況**：

IP 格式錯誤（回傳 `400`）：
```bash
curl -i "http://localhost:8080/api/v1/geoip?ip=not-an-ip"
```
```json
{"error":"invalid ip"}
```

`fields` 帶入未定義的欄位名稱（回傳 `400`）：
```bash
curl -i "http://localhost:8080/api/v1/geoip?ip=8.8.8.8&fields=foo"
```
```json
{"error":"invalid field: foo"}
```

### 批次查詢 IP 地理位置

**端點**: `POST /api/v1/geoip/batch`

**參數**:
- `lang` (可選, query string): 回應語言，支援 `en`（預設）、`zh-CN`、`ja`、`ko`、`ru`、`fr`、`de`、`es`、`pt-BR`、`fa`，套用於整批結果。
- `fields` (可選, query string): 以逗號分隔要回傳的欄位，可選 `ip`、`country`、`region`、`city`、`country_geoname_id`、`region_geoname_id`、`city_geoname_id`，套用於整批結果（查詢失敗的項目仍固定回傳 `ip` 與 `error`）。未提供時回傳所有基本欄位；帶入未知欄位會回傳 `400`。
- `translate` (可選, query string, 布林值, 預設 `false`): 同單一查詢的 `translate`，套用於整批結果。

**請求主體**（最多 100 個 IP）:
```json
{
  "ips": ["8.8.8.8", "1.1.1.1"]
}
```

**回應**:
```json
{
  "results": [
    { "ip": "8.8.8.8", "country": "United States", "region": "California", "city": "San Francisco" },
    { "ip": "1.1.1.1", "country": "Australia", "region": "Unknown", "city": "Unknown" }
  ]
}
```

若某個 IP 查詢失敗，該筆結果會改以 `error` 欄位表示，例如：
```json
{ "ip": "not-an-ip", "error": "invalid ip" }
```

**範例**:
```bash
curl -X POST "http://localhost:8080/api/v1/geoip/batch?lang=ja" \
  -H "Content-Type: application/json" \
  -d '{"ips":["8.8.8.8","1.1.1.1"]}'
```

只回傳部分欄位（`fields` 套用於整批結果，查詢失敗的項目仍固定回傳 `ip` 與 `error`）：
```bash
curl -X POST "http://localhost:8080/api/v1/geoip/batch?fields=country,city" \
  -H "Content-Type: application/json" \
  -d '{"ips":["8.8.8.8","1.1.1.1","not-an-ip"]}'
```

**錯誤情況**：

`ips` 為空陣列、超過 100 筆，或請求主體不是合法 JSON 時，整個請求回傳 `400`（單筆 IP 格式錯誤則仍是 `200`，錯誤反映在該筆結果的 `error` 欄位，見上方回應範例）：
```bash
curl -i -X POST "http://localhost:8080/api/v1/geoip/batch" \
  -H "Content-Type: application/json" \
  -d '{"ips":[]}'
```
```json
{"error":"ips must not be empty"}
```

### 健康檢查

如果啟用健康檢查（`enable_health: true`），則可以使用以下端點：

**端點**: `GET /api/v1/health`

**回應**:
```json
{
  "status": "ok",
  "db": "ok",
  "last_update": "2023-10-01 12:00:00",
  "age_hours": "24.50",
  "size": "50.00 MB"
}
```

**範例**:
```bash
curl "http://localhost:8080/api/v1/health"
```

當資料庫檔案不存在或查詢測試失敗時，`status` 會變成 `degraded`，同時回傳 HTTP `503`：
```json
{
  "status": "degraded",
  "db": "fail",
  "last_update": "",
  "age_hours": "0.00",
  "size": "0 MB"
}
```

未啟用健康檢查（`enable_health: false` 或未設定）時，此端點不會註冊，請求會得到 `404`。

### 版本資訊

**端點**: `GET /api/v1/version`

**回應**:
```json
{
  "version": "1.0.0"
}
```

**範例**:
```bash
curl "http://localhost:8080/api/v1/version"
```

## 本地翻譯對照表

MaxMind GeoLite2 / DB-IP 等資料庫（尤其是免費 / Lite 版本）通常只有 `country` 層級有完整多語言翻譯，`region`（subdivisions）與 `city` 往往只有英文名稱。當資料庫沒有對應語言的翻譯時，可以設定 `name_mapping_path` 指向一份本地維護的對照表，並在請求時帶上 `translate=true` 來套用它作為 fallback。

**優先順序**：資料庫本身的翻譯 > 本地對照表 > 英文原名。也就是說對照表只會補資料庫缺漏的部分，不會覆蓋資料庫已有的翻譯。

**對照表格式**（`name_mapping_path` 指向的 JSON 檔）以 mmdb 記錄裡的**英文名稱字串**為 key：
```json
{
  "China": { "zh-CN": "中国" },
  "Zhejiang": { "zh-CN": "浙江" },
  "Ningbo": { "zh-CN": "宁波市" }
}
```
- 第一層 key 為資料庫中的**英文原名**（必須與 mmdb `names.en` 完全一致才會命中）。
- 第二層 key 為語言代碼（同 `lang` 參數支援的語系），值為翻譯後的名稱。

> **為什麼不用 geonameid？** mmdb 的 `country`/`region`/`city` 記錄雖然理論上會帶 [GeoNames](https://www.geonames.org/) 的 `geoname_id` 欄位，但實測發現 **DB-IP City Lite 的 `region`/`city` 記錄該欄位固定是 `0`**（即使官方 schema 文件宣稱有這個欄位，Lite 版實際資料沒填）——可以用 `fields=region_geoname_id,city_geoname_id` 自行驗證你的資料庫是否也是如此。因此只能退而求其次用英文名稱字串比對，代價是少數「不同地方剛好同名」（例如國家 Georgia 和美國的 Georgia 州）可能會誤配；如果你的資料庫的 `geoname_id` 確實有填值，用該欄位比對會更準確，但目前工具沒有支援，需要你自行擴充。

專案內附上 `mapping.example.json` 作為格式範例（內容為真實查證過的 China / Zhejiang / Ningbo 對照）。

### 從 GeoNames 資料批次產生對照表

如果你已經有 GeoNames 的 [`alternateNamesV2.txt`](https://download.geonames.org/export/dump/alternateNamesV2.zip) 原始資料（約 750MB，1900 多萬筆，涵蓋全球所有語言），可以用專案內附的 `tools/genmapping` 工具批次轉換成上述精簡格式，不需要手動維護：

```bash
go run ./tools/genmapping \
  -input GeoNames/alternateNamesV2.txt \
  -out mapping.json \
  -langs zh-CN,ja,ko,ru,fr,de,es,pt-BR,fa
```

- `-input`: GeoNames `alternateNamesV2.txt` 路徑（預設 `GeoNames/alternateNamesV2.txt`）。
- `-out`: 輸出的對照表路徑（預設 `mapping.json`），完成後把 `name_mapping_path` 指向這個檔案即可。
- `-langs`: 要抽取的語言，逗號分隔（預設涵蓋除 `en` 外的所有支援語系）。

GeoNames 的資料裡中文名稱多半標記在通用的 `zh`（而非 `zh-CN`）語系代碼下，葡萄牙文也多半在 `pt` 而非 `pt-BR` 下；工具內建了這個對應關係，會優先採用精確代碼（`zh-CN`/`pt-BR`），沒有時才 fallback 到通用代碼（`zh`/`pt`）。同一個 geonameid 有多筆候選譯名時，會優先採用 GeoNames 標記的 `isPreferredName`，其次是 `isShortName`；不同 geonameid 剛好有相同英文名稱時，會優先採用翻譯候選 rank 較高者，同分則採用較小的 geonameid（通常代表較早收錄、較知名的條目），這是啟發式規則，不保證每筆都正確，重要地名建議產生後人工檢查。

**注意**：原始的 `alternateNamesV2.txt`（748MB）與跑出來的完整 `mapping.json` 都不會進版本控制（見 `.gitignore`），請自行下載/產生並部署到伺服器上，只有 `name_mapping_path` 指向的檔案需要放到部署環境即可。未設定 `name_mapping_path`（或檔案讀取失敗）時，`translate=true` 不會有任何效果，會直接回退為英文原名，且服務不會因此中斷。

## 系統服務管理 (systemd)

### 安裝為系統服務

1. **複製 systemd 服務檔案**:
```bash
sudo cp geoip.service /etc/systemd/system/
```

2. **編輯服務檔案（可選）** - 若路徑不同，需修改：
```bash
sudo nano /etc/systemd/system/geoip.service
```

服務檔案應類似：
```ini
[Unit]
Description=GeoIP Service
After=network.target

[Service]
Type=simple
User=geoip
ExecStart=/opt/geoip/bin/geoip-service -config /opt/geoip/conf/config.json
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

3. **建立專用用戶**（可選但推薦）:
```bash
sudo useradd -r -s /bin/false geoip
sudo chown -R geoip:geoip /opt/geoip
```

4. **重新載入 systemd**:
```bash
sudo systemctl daemon-reload
```

### 控制服務

**啟動服務**:
```bash
sudo systemctl start geoip
```

**停止服務**:
```bash
sudo systemctl stop geoip
```

**重啟服務**:
```bash
sudo systemctl restart geoip
```

**檢查服務狀態**:
```bash
sudo systemctl status geoip
```

**設定開機自啟動**:
```bash
sudo systemctl enable geoip
```

**禁用開機自啟動**:
```bash
sudo systemctl disable geoip
```

**即時日誌**:
```bash
sudo journalctl -u geoip -f
```

### 驗證服務正常運作

```bash
# 檢查服務是否在監聽連接埠
sudo ss -tlnp | grep geoip

# 快速測試 API
curl http://localhost:8080/api/v1/version
curl http://localhost:8080/api/v1/health
```

## 資料庫定期更新

### 自動更新方案 1: 使用 geoipupdate（MaxMind 官方工具）

**安裝**:
```bash
# Ubuntu/Debian
sudo apt-get install geoipupdate

# CentOS/RHEL
sudo yum install geoipupdate
```

**配置** - 編輯 `/etc/GeoIP.conf`：
```bash
sudo nano /etc/GeoIP.conf
```

內容應包含（需提供 MaxMind 帳號 ID 和密鑰）：
```ini
AccountID 123456
LicenseKey 1234567890abcdef
EditionIDs GeoLite2-City
DatabaseDirectory /opt/geoip/data
```

**執行更新**:
```bash
sudo geoipupdate
```

**定期更新** - 編輯 crontab：
```bash
sudo crontab -e
```

新增（每週二、五的 03:00 執行）:
```bash
0 3 * * 2,5 /usr/bin/geoipupdate -f /etc/GeoIP.conf && sudo systemctl restart geoip
```

### 自動更新方案 2: 手動下載 + cron 腳本

建立更新腳本 `/opt/geoip/scripts/update-db.sh`：
```bash
#!/bin/bash
# 下載並更新資料庫

DB_DIR="/opt/geoip/data"
BACKUP_DIR="/opt/geoip/data/backup"
TEMP_DIR="/tmp/geoip-update"

# 建立備份目錄
mkdir -p "$BACKUP_DIR"
mkdir -p "$TEMP_DIR"

# 備份現有資料庫
cp "$DB_DIR/GeoLite2-City.mmdb" "$BACKUP_DIR/GeoLite2-City.mmdb.$(date +%Y%m%d_%H%M%S)" 2>/dev/null || true

# 下載新資料庫（需替換為實際下載連結）
cd "$TEMP_DIR"
wget -q https://example.com/GeoLite2-City.mmdb -O GeoLite2-City.mmdb.new

# 驗證檔案
if [ -f "GeoLite2-City.mmdb.new" ] && [ -s "GeoLite2-City.mmdb.new" ]; then
    mv "GeoLite2-City.mmdb.new" "$DB_DIR/GeoLite2-City.mmdb"
    echo "[$(date)] 資料庫更新成功" >> /opt/geoip/logs/update.log
    # 重啟服務使新資料庫生效
    sudo systemctl restart geoip
else
    echo "[$(date)] 資料庫下載失敗" >> /opt/geoip/logs/update.log
fi

# 清理臨時檔案
rm -f "$TEMP_DIR/GeoLite2-City.mmdb.new"
```

設定執行權限並加入 cron：
```bash
chmod +x /opt/geoip/scripts/update-db.sh

# 編輯 crontab
sudo crontab -e
```

新增：
```bash
0 3 * * 2,5 /opt/geoip/scripts/update-db.sh
```

### 監控資料庫更新

檢查資料庫檔案的修改時間：
```bash
ls -lh /opt/geoip/data/GeoLite2-City.mmdb
```

查看健康檢查端點中的資料庫年份：
```bash
curl http://localhost:8080/api/v1/health | jq .last_update
```


## 依賴

- [github.com/oschwald/maxminddb-golang/v2](https://github.com/oschwald/maxminddb-golang/v2)

## 授權

請參考專案授權檔案。