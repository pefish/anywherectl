# Anywherectl

[![view examples](https://img.shields.io/badge/learn%20by-examples-0C8EC5.svg?style=for-the-badge&logo=go)](https://github.com/pefish/anywherectl)

Anywherectl is a tool to remote control anything. Enjoy it !!!

## Quick start

### Start Server

```shell script
anywherectl serve --listener-token=test_token --client-token=test
```

### Start Listener

```shell script
anywherectl listen --server-token=test_token --server-address=0.0.0.0:8181 --name=pefish --config=/path/to/config/file
```

### Exec "ls" shell

```shell script
anywherectl --listener-name=pefish --listener-token=token_test --server-token=test --server-address=0.0.0.0:8181 --action=shell --data=ls
```

## Document

[doc](https://godoc.org/github.com/pefish/anywherectl)

## Todo and Done

- [x] REGISTER command (listener -> server)
- [x] REGISTER_OK command (server -> listener)
- [x] REGISTER_FAIL command (server -> listener)
- [x] Heartbeat between server and listeners (server与listener之间的心跳机制)
- [x] Listener reconnection (listener重连server机制)
- [x] Shell command
- [x] Shell command auth between listener and client （listener对client的shell命令的权限校验）
- [ ] Shell command stream （流式shell结果，比如top命令）
- [ ] Chunk transmission (数据量大是需要分chunk传输，才能继续保证tcp连接复用)
- [ ] Download file
- [ ] Upload file

## Building the source

Building **anywherectl** requires both a Go (version 1.13 or later). Once the dependencies are installed, run

```shell script
make
```

## Security Vulnerabilities

If you discover a security vulnerability, please send an e-mail to [pefish@qq.com](mailto:pefish@qq.com). All security vulnerabilities will be promptly addressed.

## License

This project is licensed under the [Apache License](LICENSE).



