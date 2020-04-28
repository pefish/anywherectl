package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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

	//fmt.Println(packageBuf.Bytes())
	i, err := conn.Write(packageBuf.Bytes())
	if err != nil {
		return 0, fmt.Errorf("(WritePackage) write to conn err - %s", err)
	}

	return i, nil
}

func ReadPackage(conn net.Conn) (*ProtocolPackage, error) {
	headerBuf := make([]byte, 136)
	_, err := io.ReadFull(conn, headerBuf)
	if err != nil {
		return nil, fmt.Errorf("(ReadPackage) failed to read command - %s", err)
	}
	version := strings.TrimSpace(string(headerBuf[:4]))
	serverToken := strings.TrimSpace(string(headerBuf[4:36]))
	listenerName := strings.TrimSpace(string(headerBuf[36:68]))
	listenerToken := strings.TrimSpace(string(headerBuf[68:100]))
	command := strings.TrimSpace(string(headerBuf[100:132]))
	var paramsStrSize uint32
	err = binary.Read(bytes.NewReader(headerBuf[132:]), binary.BigEndian, &paramsStrSize)
	if err != nil {
		return nil, fmt.Errorf("(ReadPackage) failed to read param message length - %s", err)
	}

	params := make([][]byte, 0)
	if paramsStrSize != 0 {
		paramsBuf := make([]byte, paramsStrSize) // 读出所有参数
		_, err = io.ReadFull(conn, paramsBuf)
		if err != nil {
			return nil, fmt.Errorf("(ReadPackage) failed to read param message - %s", err)
		}
		params = bytes.Split(paramsBuf, []byte("||"))
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
