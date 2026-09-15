// Command genmapping converts GeoNames' alternateNamesV2.txt dump into the
// compact English-name-keyed JSON format that geoip-service loads via the
// name_mapping_path config option. See the repo README, section
// "本地翻譯對照表", for the full workflow and output format.
//
// Usage:
//
//	go run ./tools/genmapping -input GeoNames/alternateNamesV2.txt -out mapping.json -langs zh-CN,ja,ko,ru,fr,de,es,pt-BR,fa
//
// Download alternateNamesV2.txt from https://download.geonames.org/export/dump/alternateNamesV2.zip
//
// The output is keyed by English name rather than geonameid: some mmdb
// providers (e.g. DB-IP City Lite) leave geoname_id as 0 on subdivisions/
// city records even though the schema declares the field, so the English
// name string is the only reliably present identifier to match against.
// This means two different places that happen to share an English name
// (e.g. "Georgia" the country vs. the US state) can collide - this tool
// resolves that deterministically (rank, then smaller geonameid wins; see
// the merge step in main) but you should hand-edit the generated file for
// any specific case that matters to you.
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

const englishAlias = "en"

type candidate struct {
	name string
	rank int // higher wins: 2 = isPreferredName, 1 = isShortName, 0 = plain
}

var defaultLangs = []string{"zh-CN", "ja", "ko", "ru", "fr", "de", "es", "pt-BR", "fa"}

func main() {
	input := flag.String("input", "GeoNames/alternateNamesV2.txt", "path to GeoNames alternateNamesV2.txt")
	out := flag.String("out", "mapping.json", "output mapping JSON path")
	langsFlag := flag.String("langs", strings.Join(defaultLangs, ","), "comma-separated API language codes to extract")
	flag.Parse()

	langs := strings.Split(*langsFlag, ",")
	for i := range langs {
		langs[i] = strings.TrimSpace(langs[i])
	}

	needed := map[string]bool{englishAlias: true}
	for _, l := range langs {
		for _, alias := range geonamesAliases[l] {
			needed[alias] = true
		}
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
			byLang[isoLang] = candidate{name: name, rank: rank}
			kept++
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading input: %v", err)
	}
	log.Printf("done scanning %d lines in %s, %d geonameids have at least one candidate", lineNum, time.Since(start).Round(time.Second), len(raw))

	// Merge per-geonameid candidates into a flat, English-name-keyed table.
	// When multiple geonameids share the same English name, keep whichever
	// candidate has the higher rank for that language, tie-broken by the
	// smaller geonameid (older/lower ids tend to be the more canonical
	// GeoNames entry for well-known places). This is a heuristic, not a
	// guarantee - review the output for names you care about.
	translations := make(map[string]map[string]string) // englishName -> apiLang -> name
	bestRank := make(map[string]map[string]int)        // englishName -> apiLang -> rank of chosen candidate
	bestOwner := make(map[string]map[string]uint32)    // englishName -> apiLang -> geonameid of chosen candidate

	for geonameID, byLang := range raw {
		enCand, ok := byLang[englishAlias]
		if !ok || enCand.name == "" {
			continue
		}
		englishName := enCand.name

		for _, l := range langs {
			var best candidate
			found := false
			for _, alias := range geonamesAliases[l] {
				if c, ok := byLang[alias]; ok {
					best = c
					found = true
					break
				}
			}
			if !found {
				continue
			}

			if translations[englishName] == nil {
				translations[englishName] = make(map[string]string)
				bestRank[englishName] = make(map[string]int)
				bestOwner[englishName] = make(map[string]uint32)
			}

			curRank, exists := bestRank[englishName][l]
			curOwner := bestOwner[englishName][l]
			if !exists || best.rank > curRank || (best.rank == curRank && geonameID < curOwner) {
				translations[englishName][l] = best.name
				bestRank[englishName][l] = best.rank
				bestOwner[englishName][l] = geonameID
			}
		}
	}
	log.Printf("writing %d English-name entries to %s", len(translations), *out)

	outFile, err := os.Create(*out)
	if err != nil {
		log.Fatalf("failed to create output: %v", err)
	}
	defer outFile.Close()

	enc := json.NewEncoder(outFile)
	if err := enc.Encode(translations); err != nil {
		log.Fatalf("failed to write output: %v", err)
	}

	log.Printf("done in %s", time.Since(start).Round(time.Second))
}
