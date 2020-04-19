package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
)

func ReadCommandAndParams(conn net.Conn) (string, []string, error) {
	commandBuf := make([]byte, 32)
	_, err := io.ReadFull(conn, commandBuf)
	if err != nil {
		// 读完了会抛错EOF
		return "", nil, fmt.Errorf("failed to read command - %s", err)
	}
	commandStr := strings.TrimSpace(string(commandBuf))

	var paramMsgSize int32
	err = binary.Read(conn, binary.BigEndian, &paramMsgSize)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read param message length - %s", err)
	}
	paramsBuf := make([]byte, paramMsgSize) // 读出所有参数
	_, err = io.ReadFull(conn, paramsBuf)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read param message - %s", err)
	}
	paramMessage := string(paramsBuf)

	return commandStr, strings.Split(paramMessage, " "), nil
}
