package mcp

import (
	"context"

	"huozhi/internal/models"
)

// 本文件负责「这把密钥能干什么」的判定。
//
// 需求背景：MCP 必须保留写能力（AI 客户端要能记账），但牵丝、幕间这类消费方
// 只该取数。二者共用同一个端点，区分点只能是密钥自身的 scope——所以权限从
// 鉴权中间件一路带到工具分发这一层，在这里做唯一一次判定。

type scopeCtxKey struct{}

// WithScope 把本次请求的权限注入 ctx。传输层按中间件解析出的 scope 调用；
// 进程内直接调用（测试、未来可能的内嵌用法）必须显式声明，否则按只读处理。
func WithScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, scopeCtxKey{}, normalizeScope(scope))
}

// scopeFrom 取本次请求的权限。
//
// 缺省返回只读——失败关闭。这个默认值是刻意的：新增传输层或忘记注入时，
// 后果是「写工具不可用」而不是「悄悄拿到删账权限」。
func scopeFrom(ctx context.Context) string {
	if v, ok := ctx.Value(scopeCtxKey{}).(string); ok && v != "" {
		return normalizeScope(v)
	}
	return models.ScopeRead
}

// canWrite 是否允许执行写操作。
func canWrite(ctx context.Context) bool { return scopeFrom(ctx) == models.ScopeWrite }

func normalizeScope(s string) string {
	if s == models.ScopeWrite {
		return models.ScopeWrite
	}
	return models.ScopeRead
}

// toolRequiresWrite 判断一个工具是否属于写操作。
//
// 以 ReadOnlyHint 为唯一依据（MCP 规范里它就是给客户端看的「只读」标记），
// 注解缺失时按写处理：新加工具忘了标注，代价是只读密钥用不了它，
// 而不是让只读密钥误拿到写能力。
func toolRequiresWrite(t Tool) bool {
	return t.Annotations == nil || !t.Annotations.ReadOnlyHint
}
