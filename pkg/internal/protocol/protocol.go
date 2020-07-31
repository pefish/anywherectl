package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	go_logger "github.com/pefish/go-logger"
	"io"
	"net"
	"strings"
)

type ProtocolPackage struct {
	Version       string
	ServerToken   string
	ListenerName  string
	ListenerToken string
	Command       string
	Params        [][]byte
}

func WritePackage(conn net.Conn, p *ProtocolPackage) (int, error) {
	var packageBuf bytes.Buffer

	if p.Version == "" {
		return 0, errors.New("(WritePackage) version must be set")
	}
	if len(p.Version) > 4 {
		return 0, errors.New("(WritePackage) version too long")
	}
	versionBuf := bytes.Repeat([]byte(" "), 4)
	copy(versionBuf, p.Version)
	packageBuf.Write(versionBuf)

	if len(p.ServerToken) > 32 {
		return 0, errors.New("(WritePackage) server token too long")
	}
	serverTokenBuf := bytes.Repeat([]byte(" "), 32)
	if p.ServerToken != "" {
		copy(serverTokenBuf, p.ServerToken)
	}
	packageBuf.Write(serverTokenBuf)

	if len(p.ListenerName) > 32 {
		return 0, errors.New("(WritePackage) listener name too long")
	}
	listenerNameBuf := bytes.Repeat([]byte(" "), 32)
	if p.ListenerName != "" {
		copy(listenerNameBuf, p.ListenerName)
	}
	packageBuf.Write(listenerNameBuf)

	if len(p.ListenerToken) > 32 {
		return 0, errors.New("(WritePackage) listener token too long")
	}
	listenerTokenBuf := bytes.Repeat([]byte(" "), 32)
	if p.ListenerToken != "" {
		copy(listenerTokenBuf, p.ListenerToken)
	}
	packageBuf.Write(listenerTokenBuf)

	if p.Version == "" {
		return 0, errors.New("(WritePackage) command must be set")
	}
	if len(p.Command) > 32 {
		return 0, errors.New("(WritePackage) command too long")
	}
	commandBuf := bytes.Repeat([]byte(" "), 32)
	copy(commandBuf, p.Command)
	packageBuf.Write(commandBuf)

	var paramsSize uint32
	var paramsBytes []byte
	if p.Params == nil {
		paramsSize = 0
	} else {
		paramsBytes = bytes.Join(p.Params, []byte("||"))
		paramsSize = uint32(len(paramsBytes))
	}
	paramsSizeBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(paramsSizeBuf, paramsSize)
	packageBuf.Write(paramsSizeBuf)

	if p.Params != nil {
		packageBuf.Write(paramsBytes)
	}

	i, err := conn.Write(packageBuf.Bytes())
	if err != nil {
		return 0, fmt.Errorf("(WritePackage) write to conn err - %s", err)
	}

	return i, nil
}

func ReadPackage(conn net.Conn) (*ProtocolPackage, error) {
	headerBuf := make([]byte, 136)
	_, err := io.ReadFull(conn, headerBuf)  // 接受了足够136个字节，这里才会从阻塞中恢复
	if err != nil {
		return nil, fmt.Errorf("(ReadPackage) failed to read command - %s", err)
	}
	go_logger.Logger.DebugF("headerBuf: %v", headerBuf)
	version := strings.TrimSpace(string(headerBuf[:4]))
	go_logger.Logger.DebugF("version: %s", version)
	serverToken := strings.TrimSpace(string(headerBuf[4:36]))
	go_logger.Logger.DebugF("serverToken: %s", serverToken)
	listenerName := strings.TrimSpace(string(headerBuf[36:68]))
	go_logger.Logger.DebugF("listenerName: %s", listenerName)
	listenerToken := strings.TrimSpace(string(headerBuf[68:100]))
	go_logger.Logger.DebugF("listenerToken: %s", listenerToken)
	command := strings.TrimSpace(string(headerBuf[100:132]))
	go_logger.Logger.DebugF("command: %s", command)
	var paramsStrSize uint32
	err = binary.Read(bytes.NewReader(headerBuf[132:]), binary.BigEndian, &paramsStrSize)
	if err != nil {
		return nil, fmt.Errorf("(ReadPackage) failed to read param message length - %s", err)
	}
	go_logger.Logger.DebugF("paramsStrSize: %d", paramsStrSize)

	params := make([][]byte, 0)
	if paramsStrSize != 0 {
		paramsBuf := make([]byte, paramsStrSize) // 读出所有参数
		_, err = io.ReadFull(conn, paramsBuf)
		if err != nil {
			return nil, fmt.Errorf("(ReadPackage) failed to read param message - %s", err)
		}
		params = bytes.Split(paramsBuf, []byte("||"))
		go_logger.Logger.DebugF("params: %v", params)
	}
	return &ProtocolPackage{
		Version:       version,
		ServerToken:   serverToken,
		ListenerName:  listenerName,
		ListenerToken: listenerToken,
		Command:       command,
		Params:        params,
	}, nil
}
