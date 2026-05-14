package protocol

type Remote_NetServer_requestDebugStatus_36 struct {
	Player Entity
}

func (p *Remote_NetServer_requestDebugStatus_36) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	return nil
}

func (p *Remote_NetServer_requestDebugStatus_36) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_requestDebugStatus_36) Priority() int { return PriorityNormal }

type Remote_NetServer_debugStatusClient_37 struct {
	Value              int32
	LastClientSnapshot int32
	SnapshotsSent      int32
}

func (p *Remote_NetServer_debugStatusClient_37) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Value = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.LastClientSnapshot = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.SnapshotsSent = v
	}
	return nil
}

func (p *Remote_NetServer_debugStatusClient_37) Write(w *Writer) error {
	if err := w.WriteInt32(p.Value); err != nil {
		return err
	}
	if err := w.WriteInt32(p.LastClientSnapshot); err != nil {
		return err
	}
	if err := w.WriteInt32(p.SnapshotsSent); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_debugStatusClient_37) Priority() int { return PriorityNormal }

type Remote_NetServer_debugStatusClientUnreliable_38 struct {
	Value              int32
	LastClientSnapshot int32
	SnapshotsSent      int32
}

func (p *Remote_NetServer_debugStatusClientUnreliable_38) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Value = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.LastClientSnapshot = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.SnapshotsSent = v
	}
	return nil
}

func (p *Remote_NetServer_debugStatusClientUnreliable_38) Write(w *Writer) error {
	if err := w.WriteInt32(p.Value); err != nil {
		return err
	}
	if err := w.WriteInt32(p.LastClientSnapshot); err != nil {
		return err
	}
	if err := w.WriteInt32(p.SnapshotsSent); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_debugStatusClientUnreliable_38) Priority() int { return PriorityNormal }

type Remote_NetServer_serverPacketReliable_39 struct {
	Player   Entity
	Type     string
	Contents string
}

func (p *Remote_NetServer_serverPacketReliable_39) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
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

func (p *Remote_NetServer_serverPacketReliable_39) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	s1 := p.Type
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	s2 := p.Contents
	if err := WriteString(w, &s2); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_serverPacketReliable_39) Priority() int { return PriorityNormal }

type Remote_NetServer_serverPacketUnreliable_40 struct {
	Player   Entity
	Type     string
	Contents string
}

func (p *Remote_NetServer_serverPacketUnreliable_40) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
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

func (p *Remote_NetServer_serverPacketUnreliable_40) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	s1 := p.Type
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	s2 := p.Contents
	if err := WriteString(w, &s2); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_serverPacketUnreliable_40) Priority() int { return PriorityNormal }

type Remote_NetServer_serverBinaryPacketReliable_41 struct {
	Player   Entity
	Type     string
	Contents []byte
}

func (p *Remote_NetServer_serverBinaryPacketReliable_41) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
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

func (p *Remote_NetServer_serverBinaryPacketReliable_41) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	s1 := p.Type
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	if err := WriteBytes(w, p.Contents); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_serverBinaryPacketReliable_41) Priority() int { return PriorityNormal }

type Remote_NetServer_serverBinaryPacketUnreliable_42 struct {
	Player   Entity
	Type     string
	Contents []byte
}

func (p *Remote_NetServer_serverBinaryPacketUnreliable_42) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
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

func (p *Remote_NetServer_serverBinaryPacketUnreliable_42) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	s1 := p.Type
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	if err := WriteBytes(w, p.Contents); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_serverBinaryPacketUnreliable_42) Priority() int { return PriorityNormal }

type Remote_NetServer_clientLogicDataReliable_43 struct {
	Player  Entity
	Channel string
	Value   any
}

func (p *Remote_NetServer_clientLogicDataReliable_43) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Channel = *v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Value = v
	}
	return nil
}

func (p *Remote_NetServer_clientLogicDataReliable_43) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	s1 := p.Channel
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	if err := WriteObject(w, p.Value, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_clientLogicDataReliable_43) Priority() int { return PriorityNormal }

type Remote_NetServer_clientLogicDataUnreliable_44 struct {
	Player  Entity
	Channel string
	Value   any
}

func (p *Remote_NetServer_clientLogicDataUnreliable_44) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Channel = *v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Value = v
	}
	return nil
}

func (p *Remote_NetServer_clientLogicDataUnreliable_44) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	s1 := p.Channel
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	if err := WriteObject(w, p.Value, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_clientLogicDataUnreliable_44) Priority() int { return PriorityNormal }

type Remote_NetServer_requestBlockSnapshot_45 struct {
	Player Entity
	Pos    int32
}

func (p *Remote_NetServer_requestBlockSnapshot_45) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Pos = v
	}
	return nil
}

func (p *Remote_NetServer_requestBlockSnapshot_45) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Pos); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_requestBlockSnapshot_45) Priority() int { return PriorityNormal }

type Remote_NetServer_clientPlanSnapshot_46 struct {
	Player  Entity
	GroupId int32
	Plans   any
}

func (p *Remote_NetServer_clientPlanSnapshot_46) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.GroupId = v
	}
	if v, err := ReadClientPlans(r, r.Ctx); err != nil {
		return err
	} else {
		p.Plans = v
	}
	p.Player = nil
	return nil
}

func (p *Remote_NetServer_clientPlanSnapshot_46) Write(w *Writer) error {
	if err := w.WriteInt32(p.GroupId); err != nil {
		return err
	}
	if plans, ok := p.Plans.([]*BuildPlan); ok {
		if err := WriteClientPlans(w, plans, w.Ctx); err != nil {
			return err
		}
	} else if err := WriteClientPlans(w, nil, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_clientPlanSnapshot_46) Priority() int { return PriorityNormal }

type Remote_NetServer_clientPlanSnapshotReceived_47 struct {
	Player  Entity
	GroupId int32
	Plans   any
}

func (p *Remote_NetServer_clientPlanSnapshotReceived_47) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.GroupId = v
	}
	if v, err := ReadClientPlans(r, r.Ctx); err != nil {
		return err
	} else {
		p.Plans = v
	}
	return nil
}

func (p *Remote_NetServer_clientPlanSnapshotReceived_47) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := w.WriteInt32(p.GroupId); err != nil {
		return err
	}
	if plans, ok := p.Plans.([]*BuildPlan); ok {
		if err := WriteClientPlans(w, plans, w.Ctx); err != nil {
			return err
		}
	} else if err := WriteClientPlans(w, nil, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_clientPlanSnapshotReceived_47) Priority() int { return PriorityNormal }

type Remote_NetServer_clientSnapshot_48 struct {
	Player           Entity
	SnapshotID       int32
	UnitID           int32
	Dead             bool
	X                float32
	Y                float32
	PointerX         float32
	PointerY         float32
	Rotation         float32
	BaseRotation     float32
	XVelocity        float32
	YVelocity        float32
	Mining           Tile
	Boosting         bool
	Shooting         bool
	Chatting         bool
	Building         bool
	SelectedBlock    Block
	SelectedRotation int32
	Plans            any
	ViewX            float32
	ViewY            float32
	ViewWidth        float32
	ViewHeight       float32
}

func (p *Remote_NetServer_clientSnapshot_48) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.SnapshotID = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.UnitID = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Dead = v
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
		p.PointerX = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.PointerY = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Rotation = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.BaseRotation = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.XVelocity = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.YVelocity = v
	}
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Mining = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Boosting = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Shooting = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Chatting = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Building = v
	}
	if v, err := ReadBlock(r, r.Ctx); err != nil {
		return err
	} else {
		p.SelectedBlock = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.SelectedRotation = v
	}
	if v, err := ReadPlansQueue(r, r.Ctx); err != nil {
		return err
	} else {
		p.Plans = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.ViewX = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.ViewY = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.ViewWidth = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.ViewHeight = v
	}
	p.Player = nil
	return nil
}

func (p *Remote_NetServer_clientSnapshot_48) Write(w *Writer) error {
	if err := w.WriteInt32(p.SnapshotID); err != nil {
		return err
	}
	if err := w.WriteInt32(p.UnitID); err != nil {
		return err
	}
	if err := w.WriteBool(p.Dead); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.PointerX); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.PointerY); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Rotation); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.BaseRotation); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.XVelocity); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.YVelocity); err != nil {
		return err
	}
	if err := WriteTile(w, p.Mining); err != nil {
		return err
	}
	if err := w.WriteBool(p.Boosting); err != nil {
		return err
	}
	if err := w.WriteBool(p.Shooting); err != nil {
		return err
	}
	if err := w.WriteBool(p.Chatting); err != nil {
		return err
	}
	if err := w.WriteBool(p.Building); err != nil {
		return err
	}
	if err := WriteBlock(w, p.SelectedBlock); err != nil {
		return err
	}
	if err := w.WriteInt32(p.SelectedRotation); err != nil {
		return err
	}
	if plans, ok := p.Plans.([]*BuildPlan); ok {
		if err := WritePlansQueueNet(w, plans, w.Ctx); err != nil {
			return err
		}
	} else if err := WritePlansQueueNet(w, nil, w.Ctx); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.ViewX); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.ViewY); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.ViewWidth); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.ViewHeight); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_clientSnapshot_48) Priority() int { return PriorityNormal }

type Remote_NetServer_adminRequest_49 struct {
	Player Entity
	Other  Entity
	Action AdminAction
	Params any
}

func (p *Remote_NetServer_adminRequest_49) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Other = v
	}
	if v, err := ReadAction(r); err != nil {
		return err
	} else {
		p.Action = v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Params = v
	}
	return nil
}

func (p *Remote_NetServer_adminRequest_49) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteEntity(w, p.Other); err != nil {
		return err
	}
	if err := WriteAction(w, p.Action); err != nil {
		return err
	}
	if err := WriteObject(w, p.Params, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_NetServer_adminRequest_49) Priority() int { return PriorityNormal }

type Remote_NetServer_connectConfirm_50 struct {
	Player Entity
}

func (p *Remote_NetServer_connectConfirm_50) Read(r *Reader, _ int) error { return nil }

func (p *Remote_NetServer_connectConfirm_50) Write(w *Writer) error { return nil }

func (p *Remote_NetServer_connectConfirm_50) Priority() int { return PriorityNormal }

