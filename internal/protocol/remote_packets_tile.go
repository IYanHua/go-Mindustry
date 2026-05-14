package protocol

type Remote_Build_beginBreak_132 struct {
	Unit Unit
	Team Team
	X    int32
	Y    int32
}

func (p *Remote_Build_beginBreak_132) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.X = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Y = v
	}
	return nil
}

func (p *Remote_Build_beginBreak_132) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	if err := w.WriteInt32(p.X); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Y); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Build_beginBreak_132) Priority() int { return PriorityNormal }

type Remote_Build_beginPlace_133 struct {
	Unit        Unit
	Result      Block
	Team        Team
	X           int32
	Y           int32
	Rotation    int32
	PlaceConfig any
}

func (p *Remote_Build_beginPlace_133) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Result = v
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.X = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Y = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Rotation = v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.PlaceConfig = v
	}
	return nil
}

func (p *Remote_Build_beginPlace_133) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	if err := WriteBlock(w, p.Result); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	if err := w.WriteInt32(p.X); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Y); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Rotation); err != nil {
		return err
	}
	if err := WriteObject(w, p.PlaceConfig, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Build_beginPlace_133) Priority() int { return PriorityNormal }

type Remote_Tile_setTileBlocks_134 struct {
	Block     Block
	Team      Team
	Positions []int32
}

func (p *Remote_Tile_setTileBlocks_134) Read(r *Reader, _ int) error {
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Block = v
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	if v, err := ReadInts(r); err != nil {
		return err
	} else {
		p.Positions = v
	}
	return nil
}

func (p *Remote_Tile_setTileBlocks_134) Write(w *Writer) error {
	if err := WriteBlock(w, p.Block); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	if err := WriteInts(w, p.Positions); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setTileBlocks_134) Priority() int { return PriorityNormal }

type Remote_Tile_setTileFloors_135 struct {
	Block     Block
	Positions []int32
}

func (p *Remote_Tile_setTileFloors_135) Read(r *Reader, _ int) error {
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Block = v
	}
	if v, err := ReadInts(r); err != nil {
		return err
	} else {
		p.Positions = v
	}
	return nil
}

func (p *Remote_Tile_setTileFloors_135) Write(w *Writer) error {
	if err := WriteBlock(w, p.Block); err != nil {
		return err
	}
	if err := WriteInts(w, p.Positions); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setTileFloors_135) Priority() int { return PriorityNormal }

type Remote_Tile_setTileOverlays_136 struct {
	Block     Block
	Positions []int32
}

func (p *Remote_Tile_setTileOverlays_136) Read(r *Reader, _ int) error {
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Block = v
	}
	if v, err := ReadInts(r); err != nil {
		return err
	} else {
		p.Positions = v
	}
	return nil
}

func (p *Remote_Tile_setTileOverlays_136) Write(w *Writer) error {
	if err := WriteBlock(w, p.Block); err != nil {
		return err
	}
	if err := WriteInts(w, p.Positions); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setTileOverlays_136) Priority() int { return PriorityNormal }

type Remote_Tile_setFloor_137 struct {
	Tile    Tile
	Floor   Block
	Overlay Block
}

func (p *Remote_Tile_setFloor_137) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Floor = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Overlay = v
	}
	return nil
}

func (p *Remote_Tile_setFloor_137) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := WriteBlock(w, p.Floor); err != nil {
		return err
	}
	if err := WriteBlock(w, p.Overlay); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setFloor_137) Priority() int { return PriorityNormal }

type Remote_Tile_setOverlay_138 struct {
	Tile    Tile
	Overlay Block
}

func (p *Remote_Tile_setOverlay_138) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Overlay = v
	}
	return nil
}

func (p *Remote_Tile_setOverlay_138) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := WriteBlock(w, p.Overlay); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setOverlay_138) Priority() int { return PriorityNormal }

type Remote_Tile_removeTile_139 struct {
	Tile Tile
}

func (p *Remote_Tile_removeTile_139) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	return nil
}

func (p *Remote_Tile_removeTile_139) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_removeTile_139) Priority() int { return PriorityNormal }

type Remote_Tile_setTile_140 struct {
	Tile     Tile
	Block    Block
	Team     Team
	Rotation int32
}

func (p *Remote_Tile_setTile_140) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Block = v
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Rotation = v
	}
	return nil
}

func (p *Remote_Tile_setTile_140) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := WriteBlock(w, p.Block); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Rotation); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setTile_140) Priority() int { return PriorityNormal }

type Remote_Tile_setTeam_141 struct {
	Build Building
	Team  Team
}

func (p *Remote_Tile_setTeam_141) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	return nil
}

func (p *Remote_Tile_setTeam_141) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setTeam_141) Priority() int { return PriorityNormal }

type Remote_Tile_setTeams_142 struct {
	Positions []int32
	Team      Team
}

func (p *Remote_Tile_setTeams_142) Read(r *Reader, _ int) error {
	if v, err := ReadInts(r); err != nil {
		return err
	} else {
		p.Positions = v
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	return nil
}

func (p *Remote_Tile_setTeams_142) Write(w *Writer) error {
	if err := WriteInts(w, p.Positions); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_setTeams_142) Priority() int { return PriorityNormal }

type Remote_Tile_buildDestroyed_143 struct {
	Build Building
}

func (p *Remote_Tile_buildDestroyed_143) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_Tile_buildDestroyed_143) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_buildDestroyed_143) Priority() int { return PriorityNormal }

type Remote_Tile_buildHealthUpdate_144 struct {
	Buildings IntSeq
}

func (p *Remote_Tile_buildHealthUpdate_144) Read(r *Reader, _ int) error {
	if v, err := ReadIntSeq(r); err != nil {
		return err
	} else {
		p.Buildings = v
	}
	return nil
}

func (p *Remote_Tile_buildHealthUpdate_144) Write(w *Writer) error {
	if err := WriteIntSeq(w, p.Buildings); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Tile_buildHealthUpdate_144) Priority() int { return PriorityNormal }

type Remote_ConstructBlock_deconstructFinish_145 struct {
	Tile    Tile
	Block   Block
	Builder Unit
}

func (p *Remote_ConstructBlock_deconstructFinish_145) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Block = v
	}
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Builder = v
	}
	return nil
}

func (p *Remote_ConstructBlock_deconstructFinish_145) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := WriteBlock(w, p.Block); err != nil {
		return err
	}
	if err := WriteUnit(w, p.Builder); err != nil {
		return err
	}
	return nil
}

func (p *Remote_ConstructBlock_deconstructFinish_145) Priority() int { return PriorityNormal }

type Remote_ConstructBlock_constructFinish_146 struct {
	Tile     Tile
	Block    Block
	Builder  Unit
	Rotation int8
	Team     Team
	Config   any
}

func (p *Remote_ConstructBlock_constructFinish_146) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.Block = v
	}
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Builder = v
	}
	if v, err := r.ReadByte(); err != nil {
		return err
	} else {
		p.Rotation = int8(v)
	}
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Config = v
	}
	return nil
}

func (p *Remote_ConstructBlock_constructFinish_146) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := WriteBlock(w, p.Block); err != nil {
		return err
	}
	if err := WriteUnit(w, p.Builder); err != nil {
		return err
	}
	if err := w.WriteByte(byte(p.Rotation)); err != nil {
		return err
	}
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	if err := WriteObject(w, p.Config, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_ConstructBlock_constructFinish_146) Priority() int { return PriorityNormal }

