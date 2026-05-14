package protocol

type Remote_InputHandler_transferItemEffect_60 struct {
	Item Item
	X    float32
	Y    float32
	To   Entity
}

func (p *Remote_InputHandler_transferItemEffect_60) Read(r *Reader, _ int) error {
	if v, err := ReadItem(r, r.Ctx); err != nil {
		return err
	} else {
		p.Item = v
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
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.To = v
	}
	return nil
}

func (p *Remote_InputHandler_transferItemEffect_60) Write(w *Writer) error {
	if err := WriteItem(w, p.Item); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := WriteEntity(w, p.To); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_transferItemEffect_60) Priority() int { return PriorityNormal }

type Remote_InputHandler_takeItems_61 struct {
	Build  Building
	Item   Item
	Amount int32
	To     Unit
}

func (p *Remote_InputHandler_takeItems_61) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadItem(r, r.Ctx); err != nil {
		return err
	} else {
		p.Item = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.To = v
	}
	return nil
}

func (p *Remote_InputHandler_takeItems_61) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteItem(w, p.Item); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Amount); err != nil {
		return err
	}
	if err := WriteUnit(w, p.To); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_takeItems_61) Priority() int { return PriorityNormal }

type Remote_InputHandler_transferItemToUnit_62 struct {
	Item Item
	X    float32
	Y    float32
	To   Entity
}

func (p *Remote_InputHandler_transferItemToUnit_62) Read(r *Reader, _ int) error {
	if v, err := ReadItem(r, r.Ctx); err != nil {
		return err
	} else {
		p.Item = v
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
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.To = v
	}
	return nil
}

func (p *Remote_InputHandler_transferItemToUnit_62) Write(w *Writer) error {
	if err := WriteItem(w, p.Item); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := WriteEntity(w, p.To); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_transferItemToUnit_62) Priority() int { return PriorityNormal }

type Remote_InputHandler_setItem_63 struct {
	Build  Building
	Item   Item
	Amount int32
}

func (p *Remote_InputHandler_setItem_63) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadItem(r, r.Ctx); err != nil {
		return err
	} else {
		p.Item = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	return nil
}

func (p *Remote_InputHandler_setItem_63) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteItem(w, p.Item); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Amount); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setItem_63) Priority() int { return PriorityNormal }

type Remote_InputHandler_setItems_64 struct {
	Build Building
	Items []ItemStack
}

func (p *Remote_InputHandler_setItems_64) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadItemStacks(r, r.Ctx); err != nil {
		return err
	} else {
		p.Items = v
	}
	return nil
}

func (p *Remote_InputHandler_setItems_64) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteItemStacks(w, p.Items); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setItems_64) Priority() int { return PriorityNormal }

type Remote_InputHandler_setTileItems_65 struct {
	Item      Item
	Amount    int32
	Positions []int32
}

func (p *Remote_InputHandler_setTileItems_65) Read(r *Reader, _ int) error {
	if v, err := ReadItem(r, r.Ctx); err != nil {
		return err
	} else {
		p.Item = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	if v, err := ReadInts(r); err != nil {
		return err
	} else {
		p.Positions = v
	}
	return nil
}

func (p *Remote_InputHandler_setTileItems_65) Write(w *Writer) error {
	if err := WriteItem(w, p.Item); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Amount); err != nil {
		return err
	}
	if err := WriteInts(w, p.Positions); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setTileItems_65) Priority() int { return PriorityNormal }

type Remote_InputHandler_clearItems_66 struct {
	Build Building
}

func (p *Remote_InputHandler_clearItems_66) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_clearItems_66) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_clearItems_66) Priority() int { return PriorityNormal }

type Remote_InputHandler_setLiquid_67 struct {
	Build  Building
	Liquid Liquid
	Amount float32
}

func (p *Remote_InputHandler_setLiquid_67) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadLiquid(r, r.Ctx); err != nil {
		return err
	} else {
		p.Liquid = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	return nil
}

func (p *Remote_InputHandler_setLiquid_67) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteLiquid(w, p.Liquid); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Amount); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setLiquid_67) Priority() int { return PriorityNormal }

type Remote_InputHandler_setLiquids_68 struct {
	Build   Building
	Liquids []LiquidStack
}

func (p *Remote_InputHandler_setLiquids_68) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadLiquidStacks(r, r.Ctx); err != nil {
		return err
	} else {
		p.Liquids = v
	}
	return nil
}

func (p *Remote_InputHandler_setLiquids_68) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteLiquidStacks(w, p.Liquids); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setLiquids_68) Priority() int { return PriorityNormal }

type Remote_InputHandler_setTileLiquids_69 struct {
	Liquid    Liquid
	Amount    float32
	Positions []int32
}

func (p *Remote_InputHandler_setTileLiquids_69) Read(r *Reader, _ int) error {
	if v, err := ReadLiquid(r, r.Ctx); err != nil {
		return err
	} else {
		p.Liquid = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	if v, err := ReadInts(r); err != nil {
		return err
	} else {
		p.Positions = v
	}
	return nil
}

func (p *Remote_InputHandler_setTileLiquids_69) Write(w *Writer) error {
	if err := WriteLiquid(w, p.Liquid); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Amount); err != nil {
		return err
	}
	if err := WriteInts(w, p.Positions); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setTileLiquids_69) Priority() int { return PriorityNormal }

type Remote_InputHandler_clearLiquids_70 struct {
	Build Building
}

func (p *Remote_InputHandler_clearLiquids_70) Read(r *Reader, _ int) error {
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_clearLiquids_70) Write(w *Writer) error {
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_clearLiquids_70) Priority() int { return PriorityNormal }

type Remote_InputHandler_transferItemTo_71 struct {
	Unit   any
	Item   Item
	Amount int32
	X      float32
	Y      float32
	Build  Building
}

func (p *Remote_InputHandler_transferItemTo_71) Read(r *Reader, _ int) error {
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	if v, err := ReadItem(r, r.Ctx); err != nil {
		return err
	} else {
		p.Item = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Amount = v
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
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_transferItemTo_71) Write(w *Writer) error {
	if err := WriteObject(w, p.Unit, w.Ctx); err != nil {
		return err
	}
	if err := WriteItem(w, p.Item); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Amount); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_transferItemTo_71) Priority() int { return PriorityNormal }

type Remote_InputHandler_deletePlans_72 struct {
	Positions []int32
}

func (p *Remote_InputHandler_deletePlans_72) Read(r *Reader, _ int) error {
	if v, err := ReadInts(r); err != nil {
		return err
	} else {
		p.Positions = v
	}
	return nil
}

func (p *Remote_InputHandler_deletePlans_72) Write(w *Writer) error {
	if err := WriteInts(w, p.Positions); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_deletePlans_72) Priority() int { return PriorityNormal }

type Remote_InputHandler_pingLocation_73 struct {
	Player Entity
	X      float32
	Y      float32
	Text   any
}

func (p *Remote_InputHandler_pingLocation_73) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	x, xErr := clientReader.ReadFloat32()
	y, yErr := clientReader.ReadFloat32()
	text, textErr := ReadObject(clientReader, false, clientReader.Ctx)
	if xErr == nil && yErr == nil && textErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.X = x
		p.Y = y
		p.Text = text
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := serverReader.ReadFloat32(); err != nil {
		return err
	} else {
		p.X = v
	}
	if v, err := serverReader.ReadFloat32(); err != nil {
		return err
	} else {
		p.Y = v
	}
	if v, err := ReadObject(serverReader, false, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Text = v
	}
	return nil
}

func (p *Remote_InputHandler_pingLocation_73) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	if err := WriteObject(w, p.Text, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_pingLocation_73) Priority() int { return PriorityNormal }

type Remote_InputHandler_commandUnits_74 struct {
	Player       Entity
	UnitIds      []int32
	BuildTarget  any
	UnitTarget   any
	PosTarget    any
	QueueCommand bool
	FinalBatch   bool
}

func (p *Remote_InputHandler_commandUnits_74) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	unitIDs, idsErr := ReadInts(clientReader)
	buildTarget, buildErr := ReadBuilding(clientReader, clientReader.Ctx)
	unitTarget, unitErr := ReadUnit(clientReader, clientReader.Ctx)
	posTarget, posErr := ReadVec2(clientReader)
	queueCommand, queueErr := clientReader.ReadBool()
	finalBatch, finalErr := clientReader.ReadBool()
	if idsErr == nil && buildErr == nil && unitErr == nil && posErr == nil && queueErr == nil && finalErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.UnitIds = unitIDs
		p.BuildTarget = buildTarget
		p.UnitTarget = unitTarget
		p.PosTarget = posTarget
		p.QueueCommand = queueCommand
		p.FinalBatch = finalBatch
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadInts(serverReader); err != nil {
		return err
	} else {
		p.UnitIds = v
	}
	if v, err := ReadObject(serverReader, false, serverReader.Ctx); err != nil {
		return err
	} else {
		p.BuildTarget = v
	}
	if v, err := ReadObject(serverReader, false, serverReader.Ctx); err != nil {
		return err
	} else {
		p.UnitTarget = v
	}
	if v, err := ReadObject(serverReader, false, serverReader.Ctx); err != nil {
		return err
	} else {
		p.PosTarget = v
	}
	if v, err := serverReader.ReadBool(); err != nil {
		return err
	} else {
		p.QueueCommand = v
	}
	if v, err := serverReader.ReadBool(); err != nil {
		return err
	} else {
		p.FinalBatch = v
	}
	return nil
}

func (p *Remote_InputHandler_commandUnits_74) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteInts(w, p.UnitIds); err != nil {
		return err
	}
	if err := WriteObject(w, p.BuildTarget, w.Ctx); err != nil {
		return err
	}
	if err := WriteObject(w, p.UnitTarget, w.Ctx); err != nil {
		return err
	}
	if err := WriteObject(w, p.PosTarget, w.Ctx); err != nil {
		return err
	}
	if err := w.WriteBool(p.QueueCommand); err != nil {
		return err
	}
	if err := w.WriteBool(p.FinalBatch); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_commandUnits_74) Priority() int { return PriorityNormal }

type Remote_InputHandler_setUnitCommand_75 struct {
	Player  Entity
	UnitIds []int32
	Command *UnitCommand
}

func (p *Remote_InputHandler_setUnitCommand_75) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	unitIDs, idsErr := ReadInts(clientReader)
	command, commandErr := ReadCommand(clientReader, clientReader.Ctx)
	if idsErr == nil && commandErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.UnitIds = unitIDs
		p.Command = command
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadInts(serverReader); err != nil {
		return err
	} else {
		p.UnitIds = v
	}
	if v, err := ReadCommand(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Command = v
	}
	return nil
}

func (p *Remote_InputHandler_setUnitCommand_75) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteInts(w, p.UnitIds); err != nil {
		return err
	}
	if err := WriteCommand(w, p.Command); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setUnitCommand_75) Priority() int { return PriorityNormal }

type Remote_InputHandler_setUnitStance_76 struct {
	Player  Entity
	UnitIds []int32
	Stance  UnitStance
	Enable  bool
}

func (p *Remote_InputHandler_setUnitStance_76) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	unitIDs, idsErr := ReadInts(clientReader)
	stance, stanceErr := ReadStance(clientReader, clientReader.Ctx)
	enable, enableErr := clientReader.ReadBool()
	if idsErr == nil && stanceErr == nil && enableErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.UnitIds = unitIDs
		p.Stance = stance
		p.Enable = enable
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadInts(serverReader); err != nil {
		return err
	} else {
		p.UnitIds = v
	}
	if v, err := ReadStance(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Stance = v
	}
	if v, err := serverReader.ReadBool(); err != nil {
		return err
	} else {
		p.Enable = v
	}
	return nil
}

func (p *Remote_InputHandler_setUnitStance_76) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteInts(w, p.UnitIds); err != nil {
		return err
	}
	if err := WriteStance(w, &p.Stance); err != nil {
		return err
	}
	if err := w.WriteBool(p.Enable); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_setUnitStance_76) Priority() int { return PriorityNormal }

type Remote_InputHandler_commandBuilding_77 struct {
	Player    Entity
	Buildings []int32
	Target    Vec2
}

func (p *Remote_InputHandler_commandBuilding_77) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	buildings, buildingsErr := ReadInts(clientReader)
	target, targetErr := ReadVec2(clientReader)
	if buildingsErr == nil && targetErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Buildings = buildings
		p.Target = target
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadInts(serverReader); err != nil {
		return err
	} else {
		p.Buildings = v
	}
	if v, err := ReadVec2(serverReader); err != nil {
		return err
	} else {
		p.Target = v
	}
	return nil
}

func (p *Remote_InputHandler_commandBuilding_77) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteInts(w, p.Buildings); err != nil {
		return err
	}
	if err := WriteVec2(w, p.Target); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_commandBuilding_77) Priority() int { return PriorityNormal }

type Remote_InputHandler_requestItem_78 struct {
	Player Entity
	Build  Building
	Item   Item
	Amount int32
}

func (p *Remote_InputHandler_requestItem_78) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	build, buildErr := ReadBuilding(clientReader, clientReader.Ctx)
	item, itemErr := ReadItem(clientReader, clientReader.Ctx)
	amount, amountErr := clientReader.ReadInt32()
	if buildErr == nil && itemErr == nil && amountErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Build = build
		p.Item = item
		p.Amount = amount
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadBuilding(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadItem(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Item = v
	}
	if v, err := serverReader.ReadInt32(); err != nil {
		return err
	} else {
		p.Amount = v
	}
	return nil
}

func (p *Remote_InputHandler_requestItem_78) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteItem(w, p.Item); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Amount); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_requestItem_78) Priority() int { return PriorityNormal }

type Remote_InputHandler_transferInventory_79 struct {
	Player Entity
	Build  Building
}

func (p *Remote_InputHandler_transferInventory_79) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	build, buildErr := ReadBuilding(clientReader, clientReader.Ctx)
	if buildErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Build = build
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadBuilding(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_transferInventory_79) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_transferInventory_79) Priority() int { return PriorityNormal }

type Remote_InputHandler_removeQueueBlock_80 struct {
	X        int32
	Y        int32
	Breaking bool
}

func (p *Remote_InputHandler_removeQueueBlock_80) Read(r *Reader, _ int) error {
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
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Breaking = v
	}
	return nil
}

func (p *Remote_InputHandler_removeQueueBlock_80) Write(w *Writer) error {
	if err := w.WriteInt32(p.X); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Y); err != nil {
		return err
	}
	if err := w.WriteBool(p.Breaking); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_removeQueueBlock_80) Priority() int { return PriorityNormal }

type Remote_InputHandler_requestUnitPayload_81 struct {
	Player Entity
	Target Unit
}

func (p *Remote_InputHandler_requestUnitPayload_81) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	target, targetErr := ReadUnit(clientReader, clientReader.Ctx)
	if targetErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Target = target
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadUnit(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Target = v
	}
	return nil
}

func (p *Remote_InputHandler_requestUnitPayload_81) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteUnit(w, p.Target); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_requestUnitPayload_81) Priority() int { return PriorityNormal }

type Remote_InputHandler_requestBuildPayload_82 struct {
	Player Entity
	Build  Building
}

func (p *Remote_InputHandler_requestBuildPayload_82) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	build, buildErr := ReadBuilding(clientReader, clientReader.Ctx)
	if buildErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Build = build
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadBuilding(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_requestBuildPayload_82) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_requestBuildPayload_82) Priority() int { return PriorityNormal }

type Remote_InputHandler_pickedUnitPayload_83 struct {
	Unit   Unit
	Target Unit
}

func (p *Remote_InputHandler_pickedUnitPayload_83) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Target = v
	}
	return nil
}

func (p *Remote_InputHandler_pickedUnitPayload_83) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	if err := WriteUnit(w, p.Target); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_pickedUnitPayload_83) Priority() int { return PriorityNormal }

type Remote_InputHandler_pickedBuildPayload_84 struct {
	Unit     Unit
	Build    Building
	OnGround bool
}

func (p *Remote_InputHandler_pickedBuildPayload_84) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.OnGround = v
	}
	return nil
}

func (p *Remote_InputHandler_pickedBuildPayload_84) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := w.WriteBool(p.OnGround); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_pickedBuildPayload_84) Priority() int { return PriorityNormal }

type Remote_InputHandler_requestDropPayload_85 struct {
	Player Entity
	X      float32
	Y      float32
}

func (p *Remote_InputHandler_requestDropPayload_85) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	x, xErr := clientReader.ReadFloat32()
	y, yErr := clientReader.ReadFloat32()
	if xErr == nil && yErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.X = x
		p.Y = y
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := serverReader.ReadFloat32(); err != nil {
		return err
	} else {
		p.X = v
	}
	if v, err := serverReader.ReadFloat32(); err != nil {
		return err
	} else {
		p.Y = v
	}
	return nil
}

func (p *Remote_InputHandler_requestDropPayload_85) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_requestDropPayload_85) Priority() int { return PriorityNormal }

type Remote_InputHandler_payloadDropped_86 struct {
	Unit Unit
	X    float32
	Y    float32
}

func (p *Remote_InputHandler_payloadDropped_86) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
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
	return nil
}

func (p *Remote_InputHandler_payloadDropped_86) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.X); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Y); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_payloadDropped_86) Priority() int { return PriorityNormal }

type Remote_InputHandler_unitEnteredPayload_87 struct {
	Unit  Unit
	Build Building
}

func (p *Remote_InputHandler_unitEnteredPayload_87) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_unitEnteredPayload_87) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_unitEnteredPayload_87) Priority() int { return PriorityNormal }

type Remote_InputHandler_dropItem_88 struct {
	Player Entity
	Angle  float32
}

func (p *Remote_InputHandler_dropItem_88) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	angle, angleErr := clientReader.ReadFloat32()
	if angleErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Angle = angle
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := serverReader.ReadFloat32(); err != nil {
		return err
	} else {
		p.Angle = v
	}
	return nil
}

func (p *Remote_InputHandler_dropItem_88) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Angle); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_dropItem_88) Priority() int { return PriorityNormal }

type Remote_InputHandler_rotateBlock_89 struct {
	Player    Entity
	Build     Building
	Direction bool
}

func (p *Remote_InputHandler_rotateBlock_89) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	build, buildErr := ReadBuilding(clientReader, clientReader.Ctx)
	direction, directionErr := clientReader.ReadBool()
	if buildErr == nil && directionErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Build = build
		p.Direction = direction
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadBuilding(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := serverReader.ReadBool(); err != nil {
		return err
	} else {
		p.Direction = v
	}
	return nil
}

func (p *Remote_InputHandler_rotateBlock_89) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := w.WriteBool(p.Direction); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_rotateBlock_89) Priority() int { return PriorityNormal }

type Remote_InputHandler_tileConfig_90 struct {
	Player Entity
	Build  Building
	Value  any
}

func (p *Remote_InputHandler_tileConfig_90) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	build, buildErr := ReadBuilding(clientReader, clientReader.Ctx)
	value, valueErr := ReadObject(clientReader, false, clientReader.Ctx)
	if buildErr == nil && valueErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Build = build
		p.Value = value
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadBuilding(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	if v, err := ReadObject(serverReader, false, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Value = v
	}
	return nil
}

func (p *Remote_InputHandler_tileConfig_90) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	if err := WriteObject(w, p.Value, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_tileConfig_90) Priority() int { return PriorityNormal }

type Remote_InputHandler_tileTap_91 struct {
	Tile Tile
}

func (p *Remote_InputHandler_tileTap_91) Read(r *Reader, _ int) error {
	if v, err := ReadTile(r, r.Ctx); err != nil {
		return err
	} else {
		p.Tile = v
	}
	return nil
}

func (p *Remote_InputHandler_tileTap_91) Write(w *Writer) error {
	if err := WriteTile(w, p.Tile); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_tileTap_91) Priority() int { return PriorityNormal }

type Remote_InputHandler_buildingControlSelect_92 struct {
	Player Entity
	Build  Building
}

func (p *Remote_InputHandler_buildingControlSelect_92) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	build, buildErr := ReadBuilding(clientReader, clientReader.Ctx)
	if buildErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Build = build
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadBuilding(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_buildingControlSelect_92) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_buildingControlSelect_92) Priority() int { return PriorityNormal }

type Remote_InputHandler_unitBuildingControlSelect_93 struct {
	Unit  Unit
	Build Building
}

func (p *Remote_InputHandler_unitBuildingControlSelect_93) Read(r *Reader, _ int) error {
	if v, err := ReadUnit(r, r.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	if v, err := ReadBuilding(r, r.Ctx); err != nil {
		return err
	} else {
		p.Build = v
	}
	return nil
}

func (p *Remote_InputHandler_unitBuildingControlSelect_93) Write(w *Writer) error {
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	if err := WriteBuilding(w, p.Build); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_unitBuildingControlSelect_93) Priority() int { return PriorityNormal }

type Remote_InputHandler_unitControl_94 struct {
	Player Entity
	Unit   Unit
}

func (p *Remote_InputHandler_unitControl_94) Read(r *Reader, _ int) error {
	payload, err := r.ReadBytes(r.Remaining())
	if err != nil {
		return err
	}

	clientReader := NewReaderWithContext(payload, r.Ctx)
	unit, unitErr := ReadUnit(clientReader, clientReader.Ctx)
	if unitErr == nil && clientReader.Remaining() == 0 {
		p.Player = nil
		p.Unit = unit
		return nil
	}

	serverReader := NewReaderWithContext(payload, r.Ctx)
	if v, err := ReadEntity(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	if v, err := ReadUnit(serverReader, serverReader.Ctx); err != nil {
		return err
	} else {
		p.Unit = v
	}
	return nil
}

func (p *Remote_InputHandler_unitControl_94) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	if err := WriteUnit(w, p.Unit); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_unitControl_94) Priority() int { return PriorityNormal }

type Remote_InputHandler_unitClear_95 struct {
	Player Entity
}

func (p *Remote_InputHandler_unitClear_95) Read(r *Reader, _ int) error {
	if v, err := ReadEntity(r, r.Ctx); err != nil {
		return err
	} else {
		p.Player = v
	}
	return nil
}

func (p *Remote_InputHandler_unitClear_95) Write(w *Writer) error {
	if err := WriteEntity(w, p.Player); err != nil {
		return err
	}
	return nil
}

func (p *Remote_InputHandler_unitClear_95) Priority() int { return PriorityNormal }

