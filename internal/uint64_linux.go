//go:build (amd64 || arm64) && linux

package lumine

func toUint(n int) uint64 {
	return uint64(n)
}
