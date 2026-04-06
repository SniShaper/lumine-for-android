//go:build (arm || 386) && linux

package lumine

func toUint(n int) uint32 {
	return uint32(n)
}
