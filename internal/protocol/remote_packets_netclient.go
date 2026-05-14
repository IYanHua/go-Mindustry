package protocol

type Remote_NetClient_clientBinaryPacketReliable_5 struct {
	Type     string
	Contents []byte
}

func (p *Remote_NetClient_clientBinaryPacketReliable_5) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Type = *v
	}
	if v, err := ReadBytes(r); err != nil {
		return err
	} else {
		p.Contents = v
	}
	return nil
}

func (p *Remote_NetClient_clientBinaryPacketReliable_5) Write(w *Writer) error {
	s0 := p.Type
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	if err := WriteBytes(w, p.Contents); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_clientBinaryPacketReliable_5) Priority() int { return PriorityNormal }

type Remote_NetClient_clientBinaryPacketUnreliable_6 struct {
	Type     string
	Contents []byte
}

func (p *Remote_NetClient_clientBinaryPacketUnreliable_6) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Type = *v
	}
	if v, err := ReadBytes(r); err != nil {
		return err
	} else {
		p.Contents = v
	}
	return nil
}

func (p *Remote_NetClient_clientBinaryPacketUnreliable_6) Write(w *Writer) error {
	s0 := p.Type
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	if err := WriteBytes(w, p.Contents); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_clientBinaryPacketUnreliable_6) Priority() int { return PriorityNormal }

type Remote_NetClient_clientPacketReliable_7 struct {
	Type     string
	Contents string
}

func (p *Remote_NetClient_clientPacketReliable_7) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Type = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Contents = *v
	}
	return nil
}

func (p *Remote_NetClient_clientPacketReliable_7) Write(w *Writer) error {
	s0 := p.Type
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	s1 := p.Contents
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_clientPacketReliable_7) Priority() int { return PriorityNormal }

type Remote_NetClient_clientPacketUnreliable_8 struct {
	Type     string
	Contents string
}

func (p *Remote_NetClient_clientPacketUnreliable_8) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Type = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Contents = *v
	}
	return nil
}

func (p *Remote_NetClient_clientPacketUnreliable_8) Write(w *Writer) error {
	s0 := p.Type
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	s1 := p.Contents
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_clientPacketUnreliable_8) Priority() int { return PriorityNormal }

type Remote_NetClient_sound_9 struct {
	Sound  Sound
	Volume float32
	Pitch  float32
	Pan    float32
}

func (p *Remote_NetClient_sound_9) Read(r *Reader, _ int) error {
	if v, err := ReadSound(r, r.Ctx); err != nil {
		return err
	} else {
		p.Sound = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Volume = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Pitch = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Pan = v
	}
	return nil
}

func (p *Remote_NetClient_sound_9) Write(w *Writer) error {
	if err := WriteSound(w, p.Sound); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Volume); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Pitch); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Pan); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_sound_9) Priority() int { return PriorityNormal }

type Remote_NetClient_soundAt_10 struct {
	Sound  Sound
	X      float32
	Y      float32
	Volume float32
	Pitch  float32
}

func (p *Remote_NetClient_soundAt_10) Read(r *Reader, _ int) error {
	if v, err := ReadSound(r, r.Ctx); err != nil {
		return err
	} else {
		p.Sound = v
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
		p.Volume = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Pitch = v
	}
	return nil
}

func (p *Remote_NetClient_soundAt_10) Write(w *Writer) error {
	if err := WriteSound(w, p.Sound); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Volume); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Pitch); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_soundAt_10) Priority() int { return PriorityNormal }

type Remote_NetClient_effect_11 struct {
	Effect   Effect
	X        float32
	Y        float32
	Rotation float32
	Color    Color
}

func (p *Remote_NetClient_effect_11) Read(r *Reader, _ int) error {
	if v, err := ReadEffect(r, r.Ctx); err != nil {
		return err
	} else {
		p.Effect = v
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
		p.Rotation = v
	}
	if v, err := ReadColor(r); err != nil {
		return err
	} else {
		p.Color = v
	}
	return nil
}

func (p *Remote_NetClient_effect_11) Write(w *Writer) error {
	if err := WriteEffect(w, p.Effect); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Rotation); err != nil {
		return err
	}
	if err := WriteColor(w, p.Color); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_effect_11) Priority() int { return PriorityNormal }

type Remote_NetClient_effect_12 struct {
	Effect   Effect
	X        float32
	Y        float32
	Rotation float32
	Color    Color
	Data     any
}

func (p *Remote_NetClient_effect_12) Read(r *Reader, _ int) error {
	if v, err := ReadEffect(r, r.Ctx); err != nil {
		return err
	} else {
		p.Effect = v
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
		p.Rotation = v
	}
	if v, err := ReadColor(r); err != nil {
		return err
	} else {
		p.Color = v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Data = v
	}
	return nil
}

func (p *Remote_NetClient_effect_12) Write(w *Writer) error {
	if err := WriteEffect(w, p.Effect); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Rotation); err != nil {
		return err
	}
	if err := WriteColor(w, p.Color); err != nil {
		return err
	}
	if err := WriteObject(w, p.Data, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_effect_12) Priority() int { return PriorityNormal }

type Remote_NetClient_effectReliable_13 struct {
	Effect   Effect
	X        float32
	Y        float32
	Rotation float32
	Color    Color
}

func (p *Remote_NetClient_effectReliable_13) Read(r *Reader, _ int) error {
	if v, err := ReadEffect(r, r.Ctx); err != nil {
		return err
	} else {
		p.Effect = v
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
		p.Rotation = v
	}
	if v, err := ReadColor(r); err != nil {
		return err
	} else {
		p.Color = v
	}
	return nil
}

func (p *Remote_NetClient_effectReliable_13) Write(w *Writer) error {
	if err := WriteEffect(w, p.Effect); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Rotation); err != nil {
		return err
	}
	if err := WriteColor(w, p.Color); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_effectReliable_13) Priority() int { return PriorityNormal }

type Remote_NetClient_sendMessage_14 struct {
	Message string
}

func (p *Remote_NetClient_sendMessage_14) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	return nil
}

func (p *Remote_NetClient_sendMessage_14) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_sendMessage_14) Priority() int { return PriorityNormal }

type Remote_NetClient_sendMessage_15 struct {
	Message      string
	Unformatted  string
	Playersender Entity
}

func (p *Remote_NetClient_sendMessage_15) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Unformatted = *v
	}
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Playersender = v
	}
	return nil
}

func (p *Remote_NetClient_sendMessage_15) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	s1 := p.Unformatted
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	if err := WriteEntity(w, p.Playersender); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_sendMessage_15) Priority() int { return PriorityNormal }

type Remote_NetClient_sendChatMessage_16 struct {
	Player  Entity
	Message string
}

func (p *Remote_NetClient_sendChatMessage_16) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	p.Player = nil
	return nil
}

func (p *Remote_NetClient_sendChatMessage_16) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_sendChatMessage_16) Priority() int { return PriorityNormal }

type Remote_NetClient_connect_17 struct {
	Port int32
}

func (p *Remote_NetClient_connect_17) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Port = v
	}
	return nil
}

func (p *Remote_NetClient_connect_17) Write(w *Writer) error {
	if err := w.WriteInt32(p.Port); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_connect_17) Priority() int { return PriorityNormal }

type Remote_NetClient_ping_18 struct {
	Player Entity
	Time   int64
}

func (p *Remote_NetClient_ping_18) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := r.ReadInt64(); err != nil {
		return err
	} else {
		p.Time = v
	}
	return nil
}

func (p *Remote_NetClient_ping_18) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := w.WriteInt64(p.Time); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_ping_18) Priority() int { return PriorityNormal }

type Remote_NetClient_pingResponse_19 struct {
	Time int64
}

func (p *Remote_NetClient_pingResponse_19) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt64(); err != nil {
		return err
	} else {
		p.Time = v
	}
	return nil
}

func (p *Remote_NetClient_pingResponse_19) Write(w *Writer) error {
	if err := w.WriteInt64(p.Time); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_pingResponse_19) Priority() int { return PriorityNormal }

type Remote_NetClient_traceInfo_20 struct {
	Player Entity
	Info   TraceInfo
}

func (p *Remote_NetClient_traceInfo_20) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadTraceInfo(r); err != nil {
		return err
	} else {
		p.Info = v
	}
	return nil
}

func (p *Remote_NetClient_traceInfo_20) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteTraceInfo(w, p.Info); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_traceInfo_20) Priority() int { return PriorityNormal }

type Remote_NetClient_kick_21 struct {
	Reason KickReason
}

func (p *Remote_NetClient_kick_21) Read(r *Reader, _ int) error {
	if v, err := ReadKick(r); err != nil {
		return err
	} else {
		p.Reason = v
	}
	return nil
}

func (p *Remote_NetClient_kick_21) Write(w *Writer) error {
	if err := WriteKick(w, p.Reason); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_kick_21) Priority() int { return PriorityNormal }

type Remote_NetClient_kick_22 struct {
	Reason string
}

func (p *Remote_NetClient_kick_22) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Reason = *v
	}
	return nil
}

func (p *Remote_NetClient_kick_22) Write(w *Writer) error {
	s0 := p.Reason
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_kick_22) Priority() int { return PriorityNormal }

type Remote_NetClient_setRules_23 struct {
	Rules Rules
}

func (p *Remote_NetClient_setRules_23) Read(r *Reader, _ int) error {
	if v, err := ReadRules(r); err != nil {
		return err
	} else {
		p.Rules = v
	}
	return nil
}

func (p *Remote_NetClient_setRules_23) Write(w *Writer) error {
	if err := WriteRules(w, p.Rules); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_setRules_23) Priority() int { return PriorityNormal }

type Remote_NetClient_setRule_24 struct {
	Rule     string
	JsonData string
}

func (p *Remote_NetClient_setRule_24) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Rule = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.JsonData = *v
	}
	return nil
}

func (p *Remote_NetClient_setRule_24) Write(w *Writer) error {
	s0 := p.Rule
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	s1 := p.JsonData
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_setRule_24) Priority() int { return PriorityNormal }

type Remote_NetClient_setObjectives_25 struct {
	Executor MapObjectives
}

func (p *Remote_NetClient_setObjectives_25) Read(r *Reader, _ int) error {
	if v, err := ReadObjectives(r); err != nil {
		return err
	} else {
		p.Executor = v
	}
	return nil
}

func (p *Remote_NetClient_setObjectives_25) Write(w *Writer) error {
	if err := WriteObjectives(w, p.Executor); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_setObjectives_25) Priority() int { return PriorityNormal }

type Remote_NetClient_clearObjectives_26 struct {
}

func (p *Remote_NetClient_clearObjectives_26) Read(r *Reader, _ int) error {
	return nil
}

func (p *Remote_NetClient_clearObjectives_26) Write(w *Writer) error {
	return nil
}

func (p *Remote_NetClient_clearObjectives_26) Priority() int { return PriorityNormal }

type Remote_NetClient_completeObjective_27 struct {
	Index int32
}

func (p *Remote_NetClient_completeObjective_27) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Index = v
	}
	return nil
}

func (p *Remote_NetClient_completeObjective_27) Write(w *Writer) error {
	if err := w.WriteInt32(p.Index); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_completeObjective_27) Priority() int { return PriorityNormal }

type Remote_NetClient_worldDataBegin_28 struct {
}

func (p *Remote_NetClient_worldDataBegin_28) Read(r *Reader, _ int) error {
	return nil
}

func (p *Remote_NetClient_worldDataBegin_28) Write(w *Writer) error {
	return nil
}

func (p *Remote_NetClient_worldDataBegin_28) Priority() int { return PriorityNormal }

type Remote_NetClient_setPosition_29 struct {
	X float32
	Y float32
}

func (p *Remote_NetClient_setPosition_29) Read(r *Reader, _ int) error {
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
	return nil
}

func (p *Remote_NetClient_setPosition_29) Write(w *Writer) error {
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_setPosition_29) Priority() int { return PriorityNormal }

type Remote_NetClient_setCameraPosition_30 struct {
	X float32
	Y float32
}

func (p *Remote_NetClient_setCameraPosition_30) Read(r *Reader, _ int) error {
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
	return nil
}

func (p *Remote_NetClient_setCameraPosition_30) Write(w *Writer) error {
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_setCameraPosition_30) Priority() int { return PriorityNormal }

type Remote_NetClient_playerDisconnect_31 struct {
	Playerid int32
}

func (p *Remote_NetClient_playerDisconnect_31) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Playerid = v
	}
	return nil
}

func (p *Remote_NetClient_playerDisconnect_31) Write(w *Writer) error {
	if err := w.WriteInt32(p.Playerid); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_playerDisconnect_31) Priority() int { return PriorityNormal }

type Remote_NetClient_entitySnapshot_32 struct {
	Amount int16
	Data   []byte
}

func (p *Remote_NetClient_entitySnapshot_32) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt16(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	if v, err := ReadBytes(r); err != nil {
		return err
	} else {
		p.Data = v
	}
	return nil
}

func (p *Remote_NetClient_entitySnapshot_32) Write(w *Writer) error {
	if err := w.WriteInt16(p.Amount); err != nil {
		return err
	}
	if err := WriteBytes(w, p.Data); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_entitySnapshot_32) Priority() int { return PriorityNormal }

type Remote_NetClient_hiddenSnapshot_33 struct {
	Ids IntSeq
}

func (p *Remote_NetClient_hiddenSnapshot_33) Read(r *Reader, _ int) error {
	if v, err := ReadIntSeq(r); err != nil {
		return err
	} else {
		p.Ids = v
	}
	return nil
}

func (p *Remote_NetClient_hiddenSnapshot_33) Write(w *Writer) error {
	if err := WriteIntSeq(w, p.Ids); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_hiddenSnapshot_33) Priority() int { return PriorityNormal }

type Remote_NetClient_blockSnapshot_34 struct {
	Amount int16
	Data   []byte
}

func (p *Remote_NetClient_blockSnapshot_34) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt16(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	if v, err := ReadBytes(r); err != nil {
		return err
	} else {
		p.Data = v
	}
	return nil
}

func (p *Remote_NetClient_blockSnapshot_34) Write(w *Writer) error {
	if err := w.WriteInt16(p.Amount); err != nil {
		return err
	}
	if err := WriteBytes(w, p.Data); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_blockSnapshot_34) Priority() int { return PriorityNormal }

type Remote_NetClient_stateSnapshot_35 struct {
	WaveTime float32
	Wave     int32
	Enemies  int32
	Paused   bool
	GameOver bool
	TimeData int32
	Tps      int8
	Rand0    int64
	Rand1    int64
	CoreData []byte
}

func (p *Remote_NetClient_stateSnapshot_35) Read(r *Reader, _ int) error {
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.WaveTime = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Wave = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Enemies = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Paused = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.GameOver = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.TimeData = v
	}
	if v, err := r.ReadByte(); err != nil {
		return err
	} else {
		p.Tps = int8(v)
	}
	if v, err := r.ReadInt64(); err != nil {
		return err
	} else {
		p.Rand0 = v
	}
	if v, err := r.ReadInt64(); err != nil {
		return err
	} else {
		p.Rand1 = v
	}
	if v, err := ReadBytes(r); err != nil {
		return err
	} else {
		p.CoreData = v
	}
	return nil
}

func (p *Remote_NetClient_stateSnapshot_35) Write(w *Writer) error {
	if err := w.WriteFloat32(p.WaveTime); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Wave); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Enemies); err != nil {
		return err
	}
	if err := w.WriteBool(p.Paused); err != nil {
		return err
	}
	if err := w.WriteBool(p.GameOver); err != nil {
		return err
	}
	if err := w.WriteInt32(p.TimeData); err != nil {
		return err
	}
	if err := w.WriteByte(byte(p.Tps)); err != nil {
		return err
	}
	if err := w.WriteInt64(p.Rand0); err != nil {
		return err
	}
	if err := w.WriteInt64(p.Rand1); err != nil {
		return err
	}
	if err := WriteBytes(w, p.CoreData); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetClient_stateSnapshot_35) Priority() int { return PriorityNormal }

