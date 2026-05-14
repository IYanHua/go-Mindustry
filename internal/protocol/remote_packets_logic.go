package protocol

type Remote_Logic_sectorCapture_1 struct {
}

func (p *Remote_Logic_sectorCapture_1) Read(r *Reader, _ int) error {
	return nil
}

func (p *Remote_Logic_sectorCapture_1) Write(w *Writer) error {
	return nil
}

func (p *Remote_Logic_sectorCapture_1) Priority() int { return PriorityNormal }

type Remote_Logic_updateGameOver_2 struct {
}

func (p *Remote_Logic_updateGameOver_2) Read(r *Reader, _ int) error {
	return nil
}

func (p *Remote_Logic_updateGameOver_2) Write(w *Writer) error {
	return nil
}

func (p *Remote_Logic_updateGameOver_2) Priority() int { return PriorityNormal }

type Remote_Logic_gameOver_3 struct {
}

func (p *Remote_Logic_gameOver_3) Read(r *Reader, _ int) error {
	return nil
}

func (p *Remote_Logic_gameOver_3) Write(w *Writer) error {
	return nil
}

func (p *Remote_Logic_gameOver_3) Priority() int { return PriorityNormal }

type Remote_Logic_researched_4 struct {
	Content Content
}

func (p *Remote_Logic_researched_4) Read(r *Reader, _ int) error {
	if v, err := ReadContent(r, r.Ctx); err != nil {
		return err
	} else {
		p.Content = v
	}
	return nil
}

func (p *Remote_Logic_researched_4) Write(w *Writer) error {
	if err := WriteContent(w, p.Content); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Logic_researched_4) Priority() int { return PriorityNormal }


type Remote_LExecutor_setMapArea_96 struct {
	X int32
	Y int32
	W int32
	H int32
}

func (p *Remote_LExecutor_setMapArea_96) Read(r *Reader, _ int) error {
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
		p.W = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.H = v
	}
	return nil
}

func (p *Remote_LExecutor_setMapArea_96) Write(w *Writer) error {
	if err := w.WriteInt32(p.X); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Y); err != nil {
		return err
	}
	if err := w.WriteInt32(p.W); err != nil {
		return err
	}
	if err := w.WriteInt32(p.H); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_setMapArea_96) Priority() int { return PriorityNormal }

type Remote_LExecutor_logicExplosion_97 struct {
	Team   Team
	X      float32
	Y      float32
	Radius float32
	Damage float32
	Air    bool
	Ground bool
	Pierce bool
	Effect bool
}

func (p *Remote_LExecutor_logicExplosion_97) Read(r *Reader, _ int) error {
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
		p.Radius = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Damage = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Air = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Ground = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Pierce = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Effect = v
	}
	return nil
}

func (p *Remote_LExecutor_logicExplosion_97) Write(w *Writer) error {
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Radius); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Damage); err != nil {
		return err
	}
	if err := w.WriteBool(p.Air); err != nil {
		return err
	}
	if err := w.WriteBool(p.Ground); err != nil {
		return err
	}
	if err := w.WriteBool(p.Pierce); err != nil {
		return err
	}
	if err := w.WriteBool(p.Effect); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_logicExplosion_97) Priority() int { return PriorityNormal }

type Remote_LExecutor_syncVariable_98 struct {
	Building Building
	Variable int32
	Value    any
}

func (p *Remote_LExecutor_syncVariable_98) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Building = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Variable = v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Value = v
	}
	return nil
}

func (p *Remote_LExecutor_syncVariable_98) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Building); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Variable); err != nil {
		return err
	}
	if err := WriteObject(w, p.Value, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_syncVariable_98) Priority() int { return PriorityNormal }

type Remote_LExecutor_setFlag_99 struct {
	Flag string
	Add  bool
}

func (p *Remote_LExecutor_setFlag_99) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Flag = *v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Add = v
	}
	return nil
}

func (p *Remote_LExecutor_setFlag_99) Write(w *Writer) error {
	s0 := p.Flag
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	if err := w.WriteBool(p.Add); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_setFlag_99) Priority() int { return PriorityNormal }

type Remote_LExecutor_createMarker_100 struct {
	Id     int32
	Marker ObjectiveMarker
}

func (p *Remote_LExecutor_createMarker_100) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	if v, err := ReadObjectiveMarker(r); err != nil {
		return err
	} else {
		p.Marker = v
	}
	return nil
}

func (p *Remote_LExecutor_createMarker_100) Write(w *Writer) error {
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	if err := WriteObjectiveMarker(w, p.Marker); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_createMarker_100) Priority() int { return PriorityNormal }

type Remote_LExecutor_removeMarker_101 struct {
	Id int32
}

func (p *Remote_LExecutor_removeMarker_101) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	return nil
}

func (p *Remote_LExecutor_removeMarker_101) Write(w *Writer) error {
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_removeMarker_101) Priority() int { return PriorityNormal }

type Remote_LExecutor_updateMarker_102 struct {
	Id      int32
	Control LMarkerControl
	P1      float64
	P2      float64
	P3      float64
}

func (p *Remote_LExecutor_updateMarker_102) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	if v, err := ReadMarkerControl(r); err != nil {
		return err
	} else {
		p.Control = v
	}
	if v, err := r.ReadFloat64(); err != nil {
		return err
	} else {
		p.P1 = v
	}
	if v, err := r.ReadFloat64(); err != nil {
		return err
	} else {
		p.P2 = v
	}
	if v, err := r.ReadFloat64(); err != nil {
		return err
	} else {
		p.P3 = v
	}
	return nil
}

func (p *Remote_LExecutor_updateMarker_102) Write(w *Writer) error {
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	if err := WriteMarkerControl(w, p.Control); err != nil {
		return err
	}
	if err := w.WriteFloat64(p.P1); err != nil {
		return err
	}
	if err := w.WriteFloat64(p.P2); err != nil {
		return err
	}
	if err := w.WriteFloat64(p.P3); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_updateMarker_102) Priority() int { return PriorityNormal }

type Remote_LExecutor_updateMarkerText_103 struct {
	Id    int32
	Type  LMarkerControl
	Fetch bool
	Text  string
}

func (p *Remote_LExecutor_updateMarkerText_103) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	if v, err := ReadMarkerControl(r); err != nil {
		return err
	} else {
		p.Type = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Fetch = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Text = *v
	}
	return nil
}

func (p *Remote_LExecutor_updateMarkerText_103) Write(w *Writer) error {
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	if err := WriteMarkerControl(w, p.Type); err != nil {
		return err
	}
	if err := w.WriteBool(p.Fetch); err != nil {
		return err
	}
	s3 := p.Text
	if err := WriteString(w, &s3); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_updateMarkerText_103) Priority() int { return PriorityNormal }

type Remote_LExecutor_updateMarkerTexture_104 struct {
	Id      int32
	Texture any
}

func (p *Remote_LExecutor_updateMarkerTexture_104) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Texture = v
	}
	return nil
}

func (p *Remote_LExecutor_updateMarkerTexture_104) Write(w *Writer) error {
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	if err := WriteObject(w, p.Texture, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LExecutor_updateMarkerTexture_104) Priority() int { return PriorityNormal }

