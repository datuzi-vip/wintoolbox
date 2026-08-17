package port

import "fmt"

// ValidTCP reports whether port is a valid TCP port number (1-65535).
func ValidTCP(p uint32) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("端口必须在 1-65535 之间")
	}
	return nil
}
