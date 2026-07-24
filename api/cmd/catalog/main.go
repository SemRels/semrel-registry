package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/SemRels/semrel-registry/api/naming"
)

type registry struct {
	SchemaVersion int      `json:"schemaVersion"`
	GeneratedAt   string   `json:"generatedAt"`
	Plugins       []plugin `json:"plugins"`
}

type plugin struct {
	Namespace   string          `json:"namespace,omitempty"`
	Name        string          `json:"name"`
	Aliases     []string        `json:"aliases,omitempty"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	Homepage    string          `json:"homepage,omitempty"`
	License     string          `json:"license"`
	Category    string          `json:"category"`
	Repository  string          `json:"repository"`
	Tags        []string        `json:"tags"`
	Versions    json.RawMessage `json:"versions"`
}

func main() {
	file := flag.String("file", "../plugins.json", "catalog to normalize or validate")
	write := flag.Bool("write", false, "write canonical names and aliases")
	flag.Parse()

	data, err := os.ReadFile(*file)
	if err != nil {
		fatal(err)
	}
	var catalog registry
	if err := json.Unmarshal(data, &catalog); err != nil {
		fatal(err)
	}
	changed, err := normalizeCatalog(&catalog)
	if err != nil {
		fatal(err)
	}
	if changed && !*write {
		fatal(fmt.Errorf("%s is not canonical; run go run ./cmd/catalog -file %s -write", *file, *file))
	}
	if !*write {
		return
	}
	output, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		fatal(err)
	}
	output = append(output, '\n')
	if !bytes.Equal(data, output) {
		if err := os.WriteFile(*file, output, 0o644); err != nil {
			fatal(err)
		}
	}
}

func normalizeCatalog(catalog *registry) (bool, error) {
	changed := false
	if catalog.SchemaVersion != 2 {
		catalog.SchemaVersion = 2
		changed = true
	}
	owners := make(map[string]string)
	for i := range catalog.Plugins {
		entry := &catalog.Plugins[i]
		canonical, ok := naming.FirstPartyByRepository(entry.Repository)
		if !ok {
			continue
		}
		aliases := append([]string(nil), canonical.Aliases...)
		sort.Strings(aliases)
		if entry.Namespace != naming.FirstPartyNamespace ||
			entry.Name != canonical.Name ||
			entry.Category != canonical.Category ||
			!reflect.DeepEqual(entry.Aliases, aliases) {
			entry.Namespace = naming.FirstPartyNamespace
			entry.Name = canonical.Name
			entry.Category = canonical.Category
			entry.Aliases = aliases
			changed = true
		}
		refs := append([]string{entry.Namespace + "/" + entry.Name}, entry.Aliases...)
		for _, ref := range refs {
			key := strings.ToLower(ref)
			if owner, exists := owners[key]; exists && owner != entry.Name {
				return false, fmt.Errorf("reference %q is owned by both %s and %s", ref, owner, entry.Name)
			}
			owners[key] = entry.Name
		}
	}
	return changed, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
