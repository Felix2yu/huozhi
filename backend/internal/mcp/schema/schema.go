// Package schema 提供构造 MCP 工具入参 JSON Schema 的小助手，
// 把「写一堆嵌套 map」收敛成一行，避免 tools.go 被样板淹没。
package schema

// Str 字符串属性
func Str(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

// Num 数值属性（金额、汇率等）
func Num(desc string) map[string]any {
	return map[string]any{"type": "number", "description": desc}
}

// Int 整数属性
func Int(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

// Bool 布尔属性
func Bool(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

// Enum 枚举字符串属性
func Enum(desc string, values ...string) map[string]any {
	vs := make([]any, 0, len(values))
	for _, v := range values {
		vs = append(vs, v)
	}
	return map[string]any{"type": "string", "description": desc, "enum": vs}
}

// ArrStr 字符串数组属性
func ArrStr(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
}

// ArrNum 数值数组属性
func ArrNum(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "number"}}
}
