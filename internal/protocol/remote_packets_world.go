package protocol

type Remote_WaveSpawner_spawnEffect_0 struct {
	X        float32
	Y        float32
	Rotation float32
	U        UnitType
}

func (p *Remote_WaveSpawner_spawnEffect_0) Read(r *Reader, _ int) error {
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
		p.Rotation = v
	}
	if v, err := ReadUnitType(r, r.Ctx); err != nil {
		return err
	} else {
		p.U = v
	}
	return nil
}

func (p *Remote_WaveSpawner_spawnEffect_0) Write(w *Writer) error {
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Rotation); err != nil {
		return err
	}
	if err := WriteUnitType(w, p.U); err != nil {
		return err
	}
	return nil
}

func (p *Remote_WaveSpawner_spawnEffect_0) Priority() int { return PriorityNormal }


type Remote_Weather_createWeather_105 struct {
	Weather   Weather
	Intensity float32
	Duration  float32
	WindX     float32
	WindY     float32
}

func (p *Remote_Weather_createWeather_105) Read(r *Reader, _ int) error {
	if v, err := ReadWeather(r, r.Ctx); err != nil {
		return err
	} else {
		p.Weather = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Intensity = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.WindX = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.WindY = v
	}
	return nil
}

func (p *Remote_Weather_createWeather_105) Write(w *Writer) error {
	if err := WriteWeather(w, p.Weather); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Intensity); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.WindX); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.WindY); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Weather_createWeather_105) Priority() int { return PriorityNormal }


type Remote_LandingPad_landingPadLanded_147 struct {
	Tile Tile
}

func (p *Remote_LandingPad_landingPadLanded_147) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	return nil
}

func (p *Remote_LandingPad_landingPadLanded_147) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	return nil
}

func (p *Remote_LandingPad_landingPadLanded_147) Priority() int { return PriorityNormal }

type Remote_AutoDoor_autoDoorToggle_148 struct {
	Tile Tile
	Open bool
}

func (p *Remote_AutoDoor_autoDoorToggle_148) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Open = v
	}
	return nil
}

func (p *Remote_AutoDoor_autoDoorToggle_148) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := w.WriteBool(p.Open); err != nil {
		return err
	}
	return nil
}

func (p *Remote_AutoDoor_autoDoorToggle_148) Priority() int { return PriorityNormal }

type Remote_CoreBlock_playerSpawn_149 struct {
	Tile   Tile
	Player Entity
}

func (p *Remote_CoreBlock_playerSpawn_149) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	return nil
}

func (p *Remote_CoreBlock_playerSpawn_149) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	return nil
}

func (p *Remote_CoreBlock_playerSpawn_149) Priority() int { return PriorityNormal }

