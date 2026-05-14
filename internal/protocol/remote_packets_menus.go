package protocol

type Remote_Menus_menu_106 struct {
	MenuId  int32
	Title   string
	Message string
	Options [][]string
}

func (p *Remote_Menus_menu_106) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.MenuId = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Title = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := ReadStringArray(r); err != nil {
		return err
	} else {
		p.Options = v
	}
	return nil
}

func (p *Remote_Menus_menu_106) Write(w *Writer) error {
	if err := w.WriteInt32(p.MenuId); err != nil {
		return err
	}
	s1 := p.Title
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	s2 := p.Message
	if err := WriteString(w, &s2); err != nil {
		return err
	}
	if err := WriteStringArray(w, p.Options); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_menu_106) Priority() int { return PriorityNormal }

type Remote_Menus_followUpMenu_107 struct {
	MenuId  int32
	Title   string
	Message string
	Options [][]string
}

func (p *Remote_Menus_followUpMenu_107) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.MenuId = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Title = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := ReadStringArray(r); err != nil {
		return err
	} else {
		p.Options = v
	}
	return nil
}

func (p *Remote_Menus_followUpMenu_107) Write(w *Writer) error {
	if err := w.WriteInt32(p.MenuId); err != nil {
		return err
	}
	s1 := p.Title
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	s2 := p.Message
	if err := WriteString(w, &s2); err != nil {
		return err
	}
	if err := WriteStringArray(w, p.Options); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_followUpMenu_107) Priority() int { return PriorityNormal }

type Remote_Menus_hideFollowUpMenu_108 struct {
	MenuId int32
}

func (p *Remote_Menus_hideFollowUpMenu_108) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.MenuId = v
	}
	return nil
}

func (p *Remote_Menus_hideFollowUpMenu_108) Write(w *Writer) error {
	if err := w.WriteInt32(p.MenuId); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_hideFollowUpMenu_108) Priority() int { return PriorityNormal }

type Remote_Menus_menuChoose_109 struct {
	MenuId int32
	Option int32
}

func (p *Remote_Menus_menuChoose_109) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.MenuId = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Option = v
	}
	return nil
}

func (p *Remote_Menus_menuChoose_109) Write(w *Writer) error {
	if err := w.WriteInt32(p.MenuId); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Option); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_menuChoose_109) Priority() int { return PriorityNormal }

type Remote_Menus_textInput_110 struct {
	TextInputId int32
	Title       string
	Message     string
	TextLength  int32
	Def         string
	Numeric     bool
}

func (p *Remote_Menus_textInput_110) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.TextInputId = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Title = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.TextLength = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Def = *v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Numeric = v
	}
	return nil
}

func (p *Remote_Menus_textInput_110) Write(w *Writer) error {
	if err := w.WriteInt32(p.TextInputId); err != nil {
		return err
	}
	s1 := p.Title
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	s2 := p.Message
	if err := WriteString(w, &s2); err != nil {
		return err
	}
	if err := w.WriteInt32(p.TextLength); err != nil {
		return err
	}
	s4 := p.Def
	if err := WriteString(w, &s4); err != nil {
		return err
	}
	if err := w.WriteBool(p.Numeric); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_textInput_110) Priority() int { return PriorityNormal }

type Remote_Menus_textInput_111 struct {
	TextInputId int32
	Title       string
	Message     string
	TextLength  int32
	Def         string
	Numeric     bool
	AllowEmpty  bool
}

func (p *Remote_Menus_textInput_111) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.TextInputId = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Title = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.TextLength = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Def = *v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.Numeric = v
	}
	if v, err := r.ReadBool(); err != nil {
		return err
	} else {
		p.AllowEmpty = v
	}
	return nil
}

func (p *Remote_Menus_textInput_111) Write(w *Writer) error {
	if err := w.WriteInt32(p.TextInputId); err != nil {
		return err
	}
	s1 := p.Title
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	s2 := p.Message
	if err := WriteString(w, &s2); err != nil {
		return err
	}
	if err := w.WriteInt32(p.TextLength); err != nil {
		return err
	}
	s4 := p.Def
	if err := WriteString(w, &s4); err != nil {
		return err
	}
	if err := w.WriteBool(p.Numeric); err != nil {
		return err
	}
	if err := w.WriteBool(p.AllowEmpty); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_textInput_111) Priority() int { return PriorityNormal }

type Remote_Menus_textInputResult_112 struct {
	TextInputId int32
	Text        any
}

func (p *Remote_Menus_textInputResult_112) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.TextInputId = v
	}
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Text = v
	}
	return nil
}

func (p *Remote_Menus_textInputResult_112) Write(w *Writer) error {
	if err := w.WriteInt32(p.TextInputId); err != nil {
		return err
	}
	if err := WriteObject(w, p.Text, w.Ctx); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_textInputResult_112) Priority() int { return PriorityNormal }

type Remote_Menus_setHudText_113 struct {
	Message string
}

func (p *Remote_Menus_setHudText_113) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	return nil
}

func (p *Remote_Menus_setHudText_113) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_setHudText_113) Priority() int { return PriorityNormal }

type Remote_Menus_hideHudText_114 struct {
}

func (p *Remote_Menus_hideHudText_114) Read(r *Reader, _ int) error {
	return nil
}

func (p *Remote_Menus_hideHudText_114) Write(w *Writer) error {
	return nil
}

func (p *Remote_Menus_hideHudText_114) Priority() int { return PriorityNormal }

type Remote_Menus_setHudTextReliable_115 struct {
	Message string
}

func (p *Remote_Menus_setHudTextReliable_115) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	return nil
}

func (p *Remote_Menus_setHudTextReliable_115) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_setHudTextReliable_115) Priority() int { return PriorityNormal }

type Remote_Menus_announce_116 struct {
	Message string
}

func (p *Remote_Menus_announce_116) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	return nil
}

func (p *Remote_Menus_announce_116) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_announce_116) Priority() int { return PriorityNormal }

type Remote_Menus_infoMessage_117 struct {
	Message string
}

func (p *Remote_Menus_infoMessage_117) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	return nil
}

func (p *Remote_Menus_infoMessage_117) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_infoMessage_117) Priority() int { return PriorityNormal }

type Remote_Menus_infoPopup_118 struct {
	Message  string
	Duration float32
	Align    int32
	Top      int32
	Left     int32
	Bottom   int32
	Right    int32
}

func (p *Remote_Menus_infoPopup_118) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Align = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Top = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Left = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Bottom = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Right = v
	}
	return nil
}

func (p *Remote_Menus_infoPopup_118) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Align); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Top); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Left); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Bottom); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Right); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_infoPopup_118) Priority() int { return PriorityNormal }

type Remote_Menus_infoPopupReliable_119 struct {
	Message  string
	Duration float32
	Align    int32
	Top      int32
	Left     int32
	Bottom   int32
	Right    int32
}

func (p *Remote_Menus_infoPopupReliable_119) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Align = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Top = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Left = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Bottom = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Right = v
	}
	return nil
}

func (p *Remote_Menus_infoPopupReliable_119) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Align); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Top); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Left); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Bottom); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Right); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_infoPopupReliable_119) Priority() int { return PriorityNormal }

type Remote_Menus_infoPopup_120 struct {
	Message  string
	Id       string
	Duration float32
	Align    int32
	Top      int32
	Left     int32
	Bottom   int32
	Right    int32
}

func (p *Remote_Menus_infoPopup_120) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Id = *v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Align = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Top = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Left = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Bottom = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Right = v
	}
	return nil
}

func (p *Remote_Menus_infoPopup_120) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	s1 := p.Id
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Align); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Top); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Left); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Bottom); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Right); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_infoPopup_120) Priority() int { return PriorityNormal }

type Remote_Menus_infoPopupReliable_121 struct {
	Message  string
	Id       string
	Duration float32
	Align    int32
	Top      int32
	Left     int32
	Bottom   int32
	Right    int32
}

func (p *Remote_Menus_infoPopupReliable_121) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Id = *v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Align = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Top = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Left = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Bottom = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Right = v
	}
	return nil
}

func (p *Remote_Menus_infoPopupReliable_121) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	s1 := p.Id
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Align); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Top); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Left); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Bottom); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Right); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_infoPopupReliable_121) Priority() int { return PriorityNormal }

type Remote_Menus_label_122 struct {
	Message  any
	Id       int32
	Duration float32
	Worldx   float32
	Worldy   float32
}

func (p *Remote_Menus_label_122) Read(r *Reader, _ int) error {
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Message = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldx = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldy = v
	}
	return nil
}

func (p *Remote_Menus_label_122) Write(w *Writer) error {
	if err := WriteObject(w, p.Message, w.Ctx); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldx); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldy); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_label_122) Priority() int { return PriorityNormal }

type Remote_Menus_labelReliable_123 struct {
	Message  any
	Id       int32
	Duration float32
	Worldx   float32
	Worldy   float32
}

func (p *Remote_Menus_labelReliable_123) Read(r *Reader, _ int) error {
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Message = v
	}
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldx = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldy = v
	}
	return nil
}

func (p *Remote_Menus_labelReliable_123) Write(w *Writer) error {
	if err := WriteObject(w, p.Message, w.Ctx); err != nil {
		return err
	}
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldx); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldy); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_labelReliable_123) Priority() int { return PriorityNormal }

type Remote_Menus_label_124 struct {
	Message  any
	Duration float32
	Worldx   float32
	Worldy   float32
}

func (p *Remote_Menus_label_124) Read(r *Reader, _ int) error {
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Message = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldx = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldy = v
	}
	return nil
}

func (p *Remote_Menus_label_124) Write(w *Writer) error {
	if err := WriteObject(w, p.Message, w.Ctx); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldx); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldy); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_label_124) Priority() int { return PriorityNormal }

type Remote_Menus_labelReliable_125 struct {
	Message  any
	Duration float32
	Worldx   float32
	Worldy   float32
}

func (p *Remote_Menus_labelReliable_125) Read(r *Reader, _ int) error {
	if v, err := ReadObject(r, false, r.Ctx); err != nil {
		return err
	} else {
		p.Message = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldx = v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Worldy = v
	}
	return nil
}

func (p *Remote_Menus_labelReliable_125) Write(w *Writer) error {
	if err := WriteObject(w, p.Message, w.Ctx); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldx); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Worldy); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_labelReliable_125) Priority() int { return PriorityNormal }

type Remote_Menus_infoToast_126 struct {
	Message  string
	Duration float32
}

func (p *Remote_Menus_infoToast_126) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Message = *v
	}
	if v, err := r.ReadFloat32(); err != nil {
		return err
	} else {
		p.Duration = v
	}
	return nil
}

func (p *Remote_Menus_infoToast_126) Write(w *Writer) error {
	s0 := p.Message
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	if err := w.WriteFloat32(p.Duration); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_infoToast_126) Priority() int { return PriorityNormal }

type Remote_Menus_warningToast_127 struct {
	Unicode int32
	Text    string
}

func (p *Remote_Menus_warningToast_127) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Unicode = v
	}
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Text = *v
	}
	return nil
}

func (p *Remote_Menus_warningToast_127) Write(w *Writer) error {
	if err := w.WriteInt32(p.Unicode); err != nil {
		return err
	}
	s1 := p.Text
	if err := WriteString(w, &s1); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_warningToast_127) Priority() int { return PriorityNormal }

type Remote_Menus_openURI_128 struct {
	Uri string
}

func (p *Remote_Menus_openURI_128) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Uri = *v
	}
	return nil
}

func (p *Remote_Menus_openURI_128) Write(w *Writer) error {
	s0 := p.Uri
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_openURI_128) Priority() int { return PriorityNormal }

type Remote_Menus_copyToClipboard_129 struct {
	Text string
}

func (p *Remote_Menus_copyToClipboard_129) Read(r *Reader, _ int) error {
	if v, err := ReadString(r); err != nil {
		return err
	} else if v != nil {
		p.Text = *v
	}
	return nil
}

func (p *Remote_Menus_copyToClipboard_129) Write(w *Writer) error {
	s0 := p.Text
	if err := WriteString(w, &s0); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_copyToClipboard_129) Priority() int { return PriorityNormal }

type Remote_Menus_removeWorldLabel_130 struct {
	Id int32
}

func (p *Remote_Menus_removeWorldLabel_130) Read(r *Reader, _ int) error {
	if v, err := r.ReadInt32(); err != nil {
		return err
	} else {
		p.Id = v
	}
	return nil
}

func (p *Remote_Menus_removeWorldLabel_130) Write(w *Writer) error {
	if err := w.WriteInt32(p.Id); err != nil {
		return err
	}
	return nil
}

func (p *Remote_Menus_removeWorldLabel_130) Priority() int { return PriorityNormal }

type Remote_HudFragment_setPlayerTeamEditor_131 struct {
	Team Team
}

func (p *Remote_HudFragment_setPlayerTeamEditor_131) Read(r *Reader, _ int) error {
	if v, err := ReadTeam(r, r.Ctx); err != nil {
		return err
	} else {
		p.Team = v
	}
	return nil
}

func (p *Remote_HudFragment_setPlayerTeamEditor_131) Write(w *Writer) error {
	if err := WriteTeam(w, &p.Team); err != nil {
		return err
	}
	return nil
}

func (p *Remote_HudFragment_setPlayerTeamEditor_131) Priority() int { return PriorityNormal }

