// +build !linux,!darwin,!windows

package shell

var ErrNotImplemented = errors.New("raw: not implemented")

func ExecShell(s string) (string, error) {
	return "", ErrNotImplemented
}
