package oracle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectOfficialTraceSmoke(t *testing.T) {
	if os.Getenv("RUN_MDT_ORACLE_INTEGRATION") != "1" {
		t.Skip("set RUN_MDT_ORACLE_INTEGRATION=1 to run the official Java oracle smoke test")
	}
	if _, err := os.Stat(DefaultWindowsRoot); err != nil {
		t.Skipf("official oracle root not present: %v", err)
	}

	repoRoot := oracleRepoRoot(t)
	trace, err := CollectOfficialTrace(Scenario{
		Name:           "official-smoke",
		MapPath:        "assets/worlds/file.msav",
		Ticks:          0,
		CaptureInitial: true,
	}, OfficialTraceOptions{
		WorkspaceRoot: repoRoot,
		WorkspaceDir:  filepath.Join(t.TempDir(), "official-smoke"),
	})
	if err != nil {
		t.Fatalf("CollectOfficialTrace: %v", err)
	}
	if len(trace.Ticks) != 1 {
		t.Fatalf("expected exactly one initial tick, got %d", len(trace.Ticks))
	}
	if len(trace.Ticks[0].Tiles) == 0 {
		t.Fatal("expected official trace to contain tile snapshots")
	}
}
