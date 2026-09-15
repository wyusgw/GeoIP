// Command genmapping converts GeoNames' alternateNamesV2.txt dump into the
// compact geonameid-keyed JSON format that geoip-service loads via the
// name_mapping_path config option. See the repo README, section
// "本地翻譯對照表", for the full workflow and output format.
//
// Usage:
//
//	go run ./tools/genmapping -input GeoNames/alternateNamesV2.txt -out mapping.json -langs zh-CN,ja,ko,ru,fr,de,es,pt-BR,fa
//
// Download alternateNamesV2.txt from https://download.geonames.org/export/dump/alternateNamesV2.zip
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// geonamesAliases lists, per API language code (as used by geoip-service's
// `lang` param), which GeoNames "isolanguage" codes may hold that
// translation, in priority order. GeoNames mostly uses bare ISO 639-1 codes
// ("zh", "pt") and only sparsely uses the regional variant tags
// ("zh-CN", "pt-BR"), so both are checked, preferring the more specific one.
var geonamesAliases = map[string][]string{
	"zh-CN": {"zh-CN", "zh"},
	"ja":    {"ja"},
	"ko":    {"ko"},
	"ru":    {"ru"},
	"fr":    {"fr"},
	"de":    {"de"},
	"es":    {"es"},
	"pt-BR": {"pt-BR", "pt"},
	"fa":    {"fa"},
}

var defaultLangs = []string{"zh-CN", "ja", "ko", "ru", "fr", "de", "es", "pt-BR", "fa"}

type candidate struct {
	name     string
	rank     int // higher wins: 2 = isPreferredName, 1 = isShortName, 0 = plain
	geoNameL string
}

func main() {
	input := flag.String("input", "GeoNames/alternateNamesV2.txt", "path to GeoNames alternateNamesV2.txt")
	out := flag.String("out", "mapping.json", "output mapping JSON path")
	langsFlag := flag.String("langs", strings.Join(defaultLangs, ","), "comma-separated API language codes to extract")
	flag.Parse()

	langs := strings.Split(*langsFlag, ",")
	for i := range langs {
		langs[i] = strings.TrimSpace(langs[i])
	}

	needed := make(map[string]bool)
	for _, l := range langs {
		for _, alias := range geonamesAliases[l] {
			needed[alias] = true
		}
	}
	if len(needed) == 0 {
		log.Fatalf("no known GeoNames alias for requested langs %v (edit geonamesAliases in this tool if you need a new language)", langs)
	}

	f, err := os.Open(*input)
	if err != nil {
		log.Fatalf("failed to open input: %v", err)
	}
	defer f.Close()

	// raw[geonameid][geonamesLangCode] = best candidate seen for that code
	raw := make(map[uint32]map[string]candidate)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	start := time.Now()
	var lineNum, kept int
	for scanner.Scan() {
		lineNum++
		if lineNum%2_000_000 == 0 {
			log.Printf("processed %d lines, kept %d candidates (%s elapsed)", lineNum, kept, time.Since(start).Round(time.Second))
		}

		line := scanner.Text()
		// columns: alternateNameId, geonameid, isolanguage, alternateName,
		// isPreferredName, isShortName, isColloquial, isHistoric, from, to
		parts := strings.SplitN(line, "\t", 7)
		if len(parts) < 4 {
			continue
		}

		isoLang := parts[2]
		if !needed[isoLang] {
			continue
		}

		name := parts[3]
		if name == "" {
			continue
		}

		geonameID64, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			continue
		}
		geonameID := uint32(geonameID64)

		rank := 0
		if len(parts) > 4 && parts[4] == "1" {
			rank = 2
		} else if len(parts) > 5 && parts[5] == "1" {
			rank = 1
		}

		byLang, ok := raw[geonameID]
		if !ok {
			byLang = make(map[string]candidate)
			raw[geonameID] = byLang
		}

		if existing, ok := byLang[isoLang]; !ok || rank > existing.rank {
			byLang[isoLang] = candidate{name: name, rank: rank, geoNameL: isoLang}
			kept++
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading input: %v", err)
	}
	log.Printf("done scanning %d lines in %s, %d geonameids have at least one candidate", lineNum, time.Since(start).Round(time.Second), len(raw))

	result := make(map[uint32]map[string]string, len(raw))
	for geonameID, byLang := range raw {
		entry := make(map[string]string)
		for _, l := range langs {
			for _, alias := range geonamesAliases[l] {
				if c, ok := byLang[alias]; ok {
					entry[l] = c.name
					break
				}
			}
		}
		if len(entry) > 0 {
			result[geonameID] = entry
		}
	}
	log.Printf("writing %d geonameid entries to %s", len(result), *out)

	outFile, err := os.Create(*out)
	if err != nil {
		log.Fatalf("failed to create output: %v", err)
	}
	defer outFile.Close()

	enc := json.NewEncoder(outFile)
	if err := enc.Encode(result); err != nil {
		log.Fatalf("failed to write output: %v", err)
	}

	log.Printf("done in %s", time.Since(start).Round(time.Second))
}
