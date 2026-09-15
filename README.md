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

## 編譯

確保您已安裝 Go 1.24.0 或更高版本。

```bash
go mod tidy
go build -o geoip-service main.go
GOOS=linux GOARCH=amd64 go build -o geoip-service main.go
```

## 配置

服務使用 JSON 格式的配置檔案。預設會檢查以下路徑：

- `./config.json`
- `./config/config.json`
- `/etc/geoip/config.json`

您也可以使用 CLI 參數或環境變數指定配置檔案：

```bash
./geoip-service -config /path/to/config.json
```

或

```bash
export CONFIG=/path/to/config.json
./geoip-service
```

配置檔案範例：

```json
{
  "port": 8080,
  "mmdb_path": "/opt/geoip/data/GeoLite2-City.mmdb",
  "enable_health": true,
  "name_mapping_path": "/opt/geoip/conf/mapping.json"
}
```

- `port`: 服務監聽的端口（預設 8080）
- `mmdb_path`: MaxMind 資料庫檔案的路徑（必需）
- `enable_health`: 是否啟用健康檢查端點（預設 false）
- `name_mapping_path` (可選): 本地翻譯對照表檔案路徑，見下方「本地翻譯對照表」章節。留空則不啟用。

## 運行

1. 下載 MaxMind GeoLite2 資料庫（例如 GeoLite2-City.mmdb）
2. 將資料庫檔案放置在配置中指定的路徑
3. 運行服務：

```bash
./geoip-service
```

## API 使用

所有端點均以 `/api/v1` 為前綴。

### 查詢單一 IP 地理位置

**端點**: `GET /api/v1/geoip`

**參數**:
- `ip` (可選): 要查詢的 IP 地址。如果未提供，則使用請求者的 IP。
- `lang` (可選): 回應語言，支援 `en`（預設）、`zh-CN`、`ja`、`ko`、`ru`、`fr`、`de`、`es`、`pt-BR`、`fa`。若資料庫中無對應翻譯（例如 MaxMind GeoLite2 未內建 `ko`、`fa` 譯名），會自動回退為 `en`。
- `fields` (可選): 以逗號分隔要回傳的欄位，可選 `ip`、`country`、`region`、`city`。未提供時回傳所有欄位；帶入未知欄位會回傳 `400`。
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

### 批次查詢 IP 地理位置

**端點**: `POST /api/v1/geoip/batch`

**參數**:
- `lang` (可選, query string): 回應語言，支援 `en`（預設）、`zh-CN`、`ja`、`ko`、`ru`、`fr`、`de`、`es`、`pt-BR`、`fa`，套用於整批結果。
- `fields` (可選, query string): 以逗號分隔要回傳的欄位，可選 `ip`、`country`、`region`、`city`，套用於整批結果（查詢失敗的項目仍固定回傳 `ip` 與 `error`）。未提供時回傳所有欄位；帶入未知欄位會回傳 `400`。
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

### 版本資訊

**端點**: `GET /api/v1/version`

**回應**:
```json
{
  "version": "1.0.0"
}
```

## 本地翻譯對照表

MaxMind GeoLite2 / DB-IP 等資料庫（尤其是免費 / Lite 版本）通常只有 `country` 層級有完整多語言翻譯，`region`（subdivisions）與 `city` 往往只有英文名稱。當資料庫沒有對應語言的翻譯時，可以設定 `name_mapping_path` 指向一份本地維護的對照表，並在請求時帶上 `translate=true` 來套用它作為 fallback。

**優先順序**：資料庫本身的翻譯 > 本地對照表 > 英文原名。也就是說對照表只會補資料庫缺漏的部分，不會覆蓋資料庫已有的翻譯。

**對照表格式**（`name_mapping_path` 指向的 JSON 檔）以 **geonameid** 為 key，而不是英文名稱字串——geonameid 是 [GeoNames](https://www.geonames.org/) 的地理實體 ID，`country`/`region`/`city` 的 mmdb 記錄本身也會帶 `geoname_id` 欄位，用它比對可以避免同名地區或大小寫不一致造成誤配：
```json
{
  "1814991": { "zh-CN": "中国" },
  "1784764": { "zh-CN": "浙江" },
  "1799397": { "zh-CN": "宁波市" }
}
```
- 第一層 key 為 geonameid（字串或數字皆可，會被解析成整數）。
- 第二層 key 為語言代碼（同 `lang` 參數支援的語系），值為翻譯後的名稱。

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

GeoNames 的資料裡中文名稱多半標記在通用的 `zh`（而非 `zh-CN`）語系代碼下，葡萄牙文也多半在 `pt` 而非 `pt-BR` 下；工具內建了這個對應關係，會優先採用精確代碼（`zh-CN`/`pt-BR`），沒有時才 fallback 到通用代碼（`zh`/`pt`）。同一個 geonameid 有多筆候選譯名時，會優先採用 GeoNames 標記的 `isPreferredName`，其次是 `isShortName`。

**注意**：原始的 `alternateNamesV2.txt`（748MB）與跑出來的完整 `mapping.json` 都不會進版本控制（見 `.gitignore`），請自行下載/產生並部署到伺服器上，只有 `name_mapping_path` 指向的檔案需要放到部署環境即可。未設定 `name_mapping_path`（或檔案讀取失敗）時，`translate=true` 不會有任何效果，會直接回退為英文原名，且服務不會因此中斷。

## 系統服務

專案包含 systemd 服務檔案 `geoip.service`，可用於將服務安裝為系統服務。

1. 複製 `geoip.service` 到 `/etc/systemd/system/`
2. 重新載入 systemd：

```bash
sudo systemctl daemon-reload
```

3. 啟動服務：

```bash
sudo systemctl start geoip
```

4. 設定開機自啟動：

```bash
sudo systemctl enable geoip
```

5.  定时更新 MaxMind 資料庫可以使用 `geoipupdate` 工具，請參考以下說明。

## 定时更新安装

```bash
apt install geoipupdate
```

## 定时任务配置

```bash
crontab -e
```

```bash
0 3 * * 2,5 /usr/bin/geoipupdate -f /opt/geoip/conf/GeoIP.conf -v
0 3 * * 2,5 /usr/bin/geoipupdate
```


## 依賴

- [github.com/oschwald/maxminddb-golang/v2](https://github.com/oschwald/maxminddb-golang/v2)

## 授權

請參考專案授權檔案。