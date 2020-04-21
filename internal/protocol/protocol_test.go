package protocol

import (
	"bytes"
	"encoding/hex"
	"github.com/pefish/anywherectl/internal/test"
	"testing"
)

func TestWritePackage(t *testing.T) {
	conn := test.NewFakeConn()
	var packageBuf bytes.Buffer
	conn.WriteFunc = func(bytes []byte) (i int, err error) {
		packageBuf.Write(bytes)
		return len(bytes), nil
	}
	i, _ := WritePackage(conn, &ProtocolPackage{
		Version:       "v0.1",
		ServerToken:   "test_token",
		ListenerName:  "haha",
		ListenerToken: "",
		Command:       "PONG",
		Params:        nil,
	})
	test.Equal(t, 136, len(packageBuf.Bytes()))
	test.Equal(t, "76302e31746573745f746f6b656e2020202020202020202020202020202020202020202068616861202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020504f4e472020202020202020202020202020202020202020202020202020202000000000", hex.EncodeToString(packageBuf.Bytes()))
	test.Equal(t, 139, i)
}

func TestReadPackage(t *testing.T) {
	conn := test.NewFakeConn()
	b, _ := hex.DecodeString("76302e31746573745f746f6b656e2020202020202020202020202020202020202020202068616861202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020504f4e472020202020202020202020202020202020202020202020202020202000000000")
	reader := bytes.NewReader(b)
	conn.ReadFunc = func(bytes []byte) (i int, err error) {
		return reader.Read(bytes)
	}
	p, _ := ReadPackage(conn)
	test.Equal(t, "v0.1", p.Version)
	test.Equal(t, "test_token", p.ServerToken)
	test.Equal(t, "haha", p.ListenerName)
	test.Equal(t, "", p.ListenerToken)
	test.Equal(t, "PONG", p.Command)
	test.Equal(t, 0, len(p.Params))
}
