#!/usr/bin/env bash
# GeoIP 服務使用方式演示
#
# 展示 README「API 使用」章節列出的各項功能，方便快速上手或驗證部署。
#
# 使用方式：
#   1. 先依 README 完成編譯與設定，並啟動服務（./geoip-service）
#   2. 執行 ./examples/demo.sh，可選帶入服務位址，預設 http://localhost:8080
#
#   ./examples/demo.sh [base_url]

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"

step() {
  echo
  echo "=== $1 ==="
}

request() {
  # 用 -w 附加狀態碼，方便確認回應是否成功
  curl -sS -w '\nHTTP %{http_code}\n' "$@"
}

step "版本資訊 (GET /api/v1/version)"
request "$BASE_URL/api/v1/version"

step "查詢單一 IP (GET /api/v1/geoip?ip=8.8.8.8)"
request "$BASE_URL/api/v1/geoip?ip=8.8.8.8"

step "指定語言 (lang=zh-CN)"
request "$BASE_URL/api/v1/geoip?ip=8.8.8.8&lang=zh-CN"

step "只回傳部分欄位 (fields=country,city)"
request "$BASE_URL/api/v1/geoip?ip=8.8.8.8&fields=country,city"

step "帶出 geoname_id 供比對本地翻譯對照表 (fields=country_geoname_id,region_geoname_id,city_geoname_id)"
request "$BASE_URL/api/v1/geoip?ip=8.8.8.8&fields=country_geoname_id,region_geoname_id,city_geoname_id"

step "套用本地翻譯對照表 fallback (translate=true)"
request "$BASE_URL/api/v1/geoip?ip=8.8.8.8&lang=zh-CN&translate=true"

step "批次查詢 (POST /api/v1/geoip/batch)"
request -X POST "$BASE_URL/api/v1/geoip/batch?lang=ja" \
  -H "Content-Type: application/json" \
  -d '{"ips":["8.8.8.8","1.1.1.1"]}'

step "批次查詢含無效 IP，觀察 error 欄位"
request -X POST "$BASE_URL/api/v1/geoip/batch" \
  -H "Content-Type: application/json" \
  -d '{"ips":["8.8.8.8","not-an-ip"]}'

step "健康檢查 (GET /api/v1/health，需 enable_health: true)"
request "$BASE_URL/api/v1/health" || true
