package kit

import "strings"

// TemplateEngine 提供简单的模板渲染功能，用于构建 UI 消息。
type TemplateEngine struct{}

// Render 使用 vars 中的键值对替换模板中的占位符。
// 占位符格式：{key}
func (t *TemplateEngine) Render(tmpl string, vars map[string]string) string {
	if tmpl == "" {
		return ""
	}
	if len(vars) == 0 {
		return tmpl
	}
	// 构建替换对
	oldnew := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		oldnew = append(oldnew, "{"+k+"}", v)
	}
	return strings.NewReplacer(oldnew...).Replace(tmpl)
}
