package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const Version = "1.0.0"

const maxBatchSize = 100

const defaultLang = "en"

var supportedLangs = map[string]bool{
	"en":    true,
	"zh-CN": true,
	"ja":    true,
	"ko":    true,
	"ru":    true,
	"fr":    true,
	"de":    true,
	"es":    true,
	"pt-BR": true,
	"fa":    true,
}

func resolveLang(lang string) string {
	if supportedLangs[lang] {
		return lang
	}
	return defaultLang
}

var (
	db  *maxminddb.Reader
	cfg Config
)

type Config struct {
	Port         int    `json:"port"`
	MMDBPath     string `json:"mmdb_path"`
	EnableHealth bool   `json:"enable_health"`
}

type Response struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
	IP      string `json:"ip"`
}

type Geo struct {
	Country struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`

	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`

	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

func loadConfig() Config {
	var configPath string
	flag.StringVar(&configPath, "config", "", "config file path")
	flag.Parse()

	if configPath != "" {
		return mustLoadConfig(configPath, "cli")
	}

	if env := os.Getenv("CONFIG"); env != "" {
		return mustLoadConfig(env, "env")
	}

	paths := []string{
		"./config.json",
		"./config/config.json",
		"/etc/geoip/config.json",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return mustLoadConfig(p, "auto")
		}
	}

	log.Fatalf("no config file found (checked CLI, ENV, default paths)")
	return Config{}
}

func mustLoadConfig(path, source string) Config {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("failed to open config (%s): %v", source, err)
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		log.Fatalf("invalid config (%s): %v", source, err)
	}

	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	if cfg.MMDBPath == "" {
		log.Fatalf("mmdb_path is required (%s)", source)
	}

	log.Printf("using config (%s): %s", source, path)
	return cfg
}

func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

func safe(s string) string {
	if s == "" {
		return "Unknown"
	}
	return s
}

func nameFor(names map[string]string, lang string) string {
	if v, ok := names[lang]; ok && v != "" {
		return v
	}
	return names[defaultLang]
}

func isDBHealthy() bool {
	if db == nil {
		return false
	}

	addr, err := netip.ParseAddr("8.8.8.8")
	if err != nil {
		return false
	}

	var g Geo
	return db.Lookup(addr).Decode(&g) == nil
}

func getDBInfo(path string) (bool, string, string, float64) {
	info, err := os.Stat(path)
	if err != nil {
		return false, "0 MB", "", 0
	}

	sizeMB := float64(info.Size()) / 1024 / 1024
	age := time.Since(info.ModTime()).Hours()

	return true,
		fmt.Sprintf("%.2f MB", sizeMB),
		info.ModTime().Format("2006-01-02 15:04:05"),
		age
}

func lookupIP(ipStr, lang string) (Response, error) {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return Response{}, fmt.Errorf("invalid ip")
	}

	var g Geo
	if err := db.Lookup(addr).Decode(&g); err != nil {
		return Response{}, fmt.Errorf("lookup failed")
	}

	res := Response{
		Country: safe(nameFor(g.Country.Names, lang)),
		City:    safe(nameFor(g.City.Names, lang)),
		Region:  "Unknown",
		IP:      ipStr,
	}

	if len(g.Subdivisions) > 0 {
		res.Region = safe(nameFor(g.Subdivisions[0].Names, lang))
	}

	return res, nil
}

func geoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ipStr := r.URL.Query().Get("ip")
	if ipStr == "" {
		ipStr = getClientIP(r)
	}

	lang := resolveLang(r.URL.Query().Get("lang"))

	res, err := lookupIP(ipStr, lang)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "invalid ip" {
			status = http.StatusBadRequest
		}
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), status)
		return
	}

	_ = json.NewEncoder(w).Encode(res)
}

type BatchRequest struct {
	IPs []string `json:"ips"`
}

type BatchResult struct {
	IP      string `json:"ip"`
	Country string `json:"country,omitempty"`
	Region  string `json:"region,omitempty"`
	City    string `json:"city,omitempty"`
	Error   string `json:"error,omitempty"`
}

func batchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if len(req.IPs) == 0 {
		http.Error(w, `{"error":"ips must not be empty"}`, http.StatusBadRequest)
		return
	}

	if len(req.IPs) > maxBatchSize {
		http.Error(w, fmt.Sprintf(`{"error":"too many ips, max %d"}`, maxBatchSize), http.StatusBadRequest)
		return
	}

	lang := resolveLang(r.URL.Query().Get("lang"))

	results := make([]BatchResult, len(req.IPs))
	for i, ipStr := range req.IPs {
		res, err := lookupIP(ipStr, lang)
		if err != nil {
			results[i] = BatchResult{IP: ipStr, Error: err.Error()}
			continue
		}
		results[i] = BatchResult{IP: res.IP, Country: res.Country, Region: res.Region, City: res.City}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"version": Version})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	exists, size, modTime, age := getDBInfo(cfg.MMDBPath)
	dbOK := isDBHealthy()

	status := "ok"
	if !exists || !dbOK {
		status = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	resp := map[string]interface{}{
		"status":      status,
		"db":          map[bool]string{true: "ok", false: "fail"}[dbOK],
		"last_update": modTime,
		"age_hours":   fmt.Sprintf("%.2f", age),
		"size":        size,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	cfg = loadConfig()

	var err error
	db, err = maxminddb.Open(cfg.MMDBPath)
	if err != nil {
		log.Fatalf("failed to open mmdb: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/geoip", geoHandler)
	mux.HandleFunc("/api/v1/geoip/batch", batchHandler)
	mux.HandleFunc("/api/v1/version", versionHandler)

	if cfg.EnableHealth {
		mux.HandleFunc("/api/v1/health", healthHandler)
	}

	addr := ":" + strconv.Itoa(cfg.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("service started on %s", addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
