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
  "enable_health": true
}
```

- `port`: 服務監聽的端口（預設 8080）
- `mmdb_path`: MaxMind 資料庫檔案的路徑（必需）
- `enable_health`: 是否啟用健康檢查端點（預設 false）

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