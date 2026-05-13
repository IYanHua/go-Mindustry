package persist

import (
	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/world"
)

func SaveMSAVSnapshot(cfg config.PersistConfig, mapPath string, snap world.Snapshot) error {
	return nil
}

func SaveMSAVSnapshotFromModel(cfg config.PersistConfig, snap world.Snapshot, model *world.WorldModel, fallbackMapPath string) error {
	return nil
}
