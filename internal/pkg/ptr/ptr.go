package ptr

func Of[T any](v T) *T {
	return &v
}

func Deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

func DerefStr(p *string) string { return Deref(p, "") }
func DerefBool(p *bool) bool     { return Deref(p, false) }
func DerefInt64(p *int64) int64  { return Deref(p, 0) }
