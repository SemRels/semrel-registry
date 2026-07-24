package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckedInCatalogIsCanonical(t *testing.T) {
	data, err := os.ReadFile("../../../plugins.json")
	require.NoError(t, err)
	var catalog registry
	require.NoError(t, json.Unmarshal(data, &catalog))
	changed, err := normalizeCatalog(&catalog)
	require.NoError(t, err)
	require.False(t, changed)
}
