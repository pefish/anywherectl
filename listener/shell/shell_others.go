// +build !linux,!darwin,!windows

package shell

var ErrNotImplemented = errors.New("shell: not implemented")

func GetCmd(s string) (*exec.Cmd, error) {
	return nil, ErrNotImplemented
}