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

### 查詢 IP 地理位置

**端點**: `GET /`

**參數**:
- `ip` (可選): 要查詢的 IP 地址。如果未提供，則使用請求者的 IP。

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
curl "http://localhost:8080/?ip=8.8.8.8"
```

### 健康檢查

如果啟用健康檢查（`enable_health: true`），則可以使用以下端點：

**端點**: `GET /health`

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

## 依賴

- [github.com/oschwald/maxminddb-golang/v2](https://github.com/oschwald/maxminddb-golang/v2)

## 授權

請參考專案授權檔案。