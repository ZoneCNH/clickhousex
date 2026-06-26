package clickhousex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlan008READMEContractDocumentsProductionDDL(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)

	for _, token := range []string{
		"ReplicatedMergeTree",
		"TTL event_time + INTERVAL 730 DAY",
		"toYYYYMM(event_time)",
		"DROP PARTITION",
		"system.parts",
	} {
		if !strings.Contains(readme, token) {
			t.Fatalf("README.md must document %q", token)
		}
	}
}
