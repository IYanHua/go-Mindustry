package world_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/IYanHua/mdt-server/internal/sim"
	"github.com/IYanHua/mdt-server/internal/world"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

func BenchmarkWorldStepCurrentMap(b *testing.B) {
	benchmarkWorldStepCurrentMap(b, false)
}

func BenchmarkWorldStepCurrentMapScheduler(b *testing.B) {
	benchmarkWorldStepCurrentMap(b, true)
}

func benchmarkWorldStepCurrentMap(b *testing.B, scheduler bool) {
	root := filepath.Join("..", "..")
	mapPath := filepath.Join(root, "assets", "worlds", "22908.msav")
	if _, err := os.Stat(mapPath); err != nil {
		b.Skipf("current map not available: %v", err)
	}
	wld := world.New(world.Config{TPS: 120})
	profilesPath := filepath.Join(root, "data", "vanilla", "profiles.json")
	if _, err := os.Stat(profilesPath); err == nil {
		if err := wld.LoadVanillaProfiles(profilesPath); err != nil {
			b.Fatalf("load vanilla profiles: %v", err)
		}
	}
	if scheduler {
		prevProcs := runtime.GOMAXPROCS(0)
		engine := sim.NewEngine(sim.Config{TPS: 120, Cores: 6, Partitions: 4})
		wld.SetScheduler(engine)
		b.Cleanup(func() {
			wld.SetScheduler(nil)
			runtime.GOMAXPROCS(prevProcs)
		})
	}
	model, err := worldstream.LoadWorldModelFromMSAV(mapPath, nil)
	if err != nil {
		b.Fatalf("load map: %v", err)
	}
	wld.SetModel(model)
	for i := 0; i < 20; i++ {
		wld.Step(time.Second / 120)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wld.Step(time.Second / 120)
	}
}
