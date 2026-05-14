package protocol

type Remote_Units_unitSpawn_51 struct {
	Container UnitSyncContainer
}

func (p *Remote_Units_unitSpawn_51) Read(r *Reader, _ int) error {
	if v, err := ReadUnitContainerBox(r, r.Ctx); err != nil {
		return err
	} else {
		p.Container = v
	}
	return nil
}

func (p *Remote_Units_unitSpawn_51) Write(w *Writer) error {
	if err := WriteUnitContainer(w, p.Container); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Units_unitSpawn_51) Priority() int { return PriorityNormal }

type Remote_Units_unitCapDeath_52 struct {
	Unit Unit
}

func (p *Remote_Units_unitCapDeath_52) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	return nil
}

func (p *Remote_Units_unitCapDeath_52) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Units_unitCapDeath_52) Priority() int { return PriorityNormal }

type Remote_Units_unitEnvDeath_53 struct {
	Unit Unit
}

func (p *Remote_Units_unitEnvDeath_53) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	return nil
}

func (p *Remote_Units_unitEnvDeath_53) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Units_unitEnvDeath_53) Priority() int { return PriorityNormal }

type Remote_Units_unitDeath_54 struct {
	Uid int32
}

func (p *Remote_Units_unitDeath_54) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Uid = v
	}
	return nil
}

func (p *Remote_Units_unitDeath_54) Write(w *Writer) error {
	if err := w.WriteInt32(p.Uid); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Units_unitDeath_54) Priority() int { return PriorityNormal }

type Remote_Units_unitDestroy_55 struct {
	Uid int32
}

func (p *Remote_Units_unitDestroy_55) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Uid = v
	}
	return nil
}

func (p *Remote_Units_unitDestroy_55) Write(w *Writer) error {
	if err := w.WriteInt32(p.Uid); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Units_unitDestroy_55) Priority() int { return PriorityNormal }

type Remote_Units_unitDespawn_56 struct {
	Unit Unit
}

func (p *Remote_Units_unitDespawn_56) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	return nil
}

func (p *Remote_Units_unitDespawn_56) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Units_unitDespawn_56) Priority() int { return PriorityNormal }

type Remote_Units_unitSafeDeath_57 struct {
	Unit Unit
}

func (p *Remote_Units_unitSafeDeath_57) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	return nil
}

func (p *Remote_Units_unitSafeDeath_57) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Units_unitSafeDeath_57) Priority() int { return PriorityNormal }

type Remote_BulletType_createBullet_58 struct {
	Type        BulletType
	Team        Team
	X           float32
	Y           float32
	Angle       float32
	Damage      float32
	VelocityScl float32
	LifetimeScl float32
}

func (p *Remote_BulletType_createBullet_58) Read(r *Reader, _ int) error {
	if v, err := ReadBulletType(r, r.Ctx); err != nil {
		return err
	} else {
		p.Type = v
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.X = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Y = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Angle = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Damage = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.VelocityScl = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.LifetimeScl = v
	}
	return nil
}

func (p *Remote_BulletType_createBullet_58) Write(w *Writer) error {
	if err := WriteBulletType(w, p.Type); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Angle); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Damage); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.VelocityScl); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.LifetimeScl); err != nil {
		return err
	}
	return nil
}

func (p *Remote_BulletType_createBullet_58) Priority() int { return PriorityNormal }

type Remote_Teams_destroyPayload_59 struct {
	Build Building
}

func (p *Remote_Teams_destroyPayload_59) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_Teams_destroyPayload_59) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Teams_destroyPayload_59) Priority() int { return PriorityNormal }


type Remote_UnitAssembler_assemblerUnitSpawned_150 struct {
	Tile Tile
}

func (p *Remote_UnitAssembler_assemblerUnitSpawned_150) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	return nil
}

func (p *Remote_UnitAssembler_assemblerUnitSpawned_150) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	return nil
}

func (p *Remote_UnitAssembler_assemblerUnitSpawned_150) Priority() int { return PriorityNormal }

type Remote_UnitAssembler_assemblerDroneSpawned_151 struct {
	Tile Tile
	Id   int32
}

func (p *Remote_UnitAssembler_assemblerDroneSpawned_151) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	return nil
}

func (p *Remote_UnitAssembler_assemblerDroneSpawned_151) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	return nil
}

func (p *Remote_UnitAssembler_assemblerDroneSpawned_151) Priority() int { return PriorityNormal }

type Remote_UnitBlock_unitBlockSpawn_152 struct {
	Tile Tile
}

func (p *Remote_UnitBlock_unitBlockSpawn_152) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	return nil
}

func (p *Remote_UnitBlock_unitBlockSpawn_152) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	return nil
}

func (p *Remote_UnitBlock_unitBlockSpawn_152) Priority() int { return PriorityNormal }

type Remote_UnitCargoLoader_unitTetherBlockSpawned_153 struct {
	Tile Tile
	Id   int32
}

func (p *Remote_UnitCargoLoader_unitTetherBlockSpawned_153) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	return nil
}

func (p *Remote_UnitCargoLoader_unitTetherBlockSpawned_153) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	return nil
}

func (p *Remote_UnitCargoLoader_unitTetherBlockSpawned_153) Priority() int { return PriorityNormal }

