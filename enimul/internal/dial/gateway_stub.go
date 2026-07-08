//go:build !windows

package dial

func getGatewayForInterface(_ int) string {
	return ""
}
