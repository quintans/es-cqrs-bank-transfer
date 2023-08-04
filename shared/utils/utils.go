package utils

type LazyStr func() string

func (s LazyStr) String() string {
	return s()
}
