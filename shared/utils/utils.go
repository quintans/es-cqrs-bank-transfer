package utils

type LazyStr struct {
	fn func() string
}

func NewLazyStr(fn func() string) LazyStr {
	return LazyStr{
		fn: fn,
	}
}

func (s LazyStr) String() string {
	return s.fn()
}
