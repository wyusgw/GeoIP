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

func geoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ipStr := r.URL.Query().Get("ip")
	if ipStr == "" {
		ipStr = getClientIP(r)
	}

	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		http.Error(w, `{"error":"invalid ip"}`, http.StatusBadRequest)
		return
	}

	var g Geo
	if err := db.Lookup(addr).Decode(&g); err != nil {
		http.Error(w, `{"error":"lookup failed"}`, http.StatusInternalServerError)
		return
	}

	res := Response{
		Country: safe(g.Country.Names["en"]),
		City:    safe(g.City.Names["en"]),
		Region:  "Unknown",
		IP:      ipStr,
	}

	if len(g.Subdivisions) > 0 {
		res.Region = safe(g.Subdivisions[0].Names["en"])
	}

	_ = json.NewEncoder(w).Encode(res)
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
	mux.HandleFunc("/", geoHandler)

	if cfg.EnableHealth {
		mux.HandleFunc("/health", healthHandler)
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
