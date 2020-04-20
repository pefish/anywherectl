# Anywherectl

[![view examples](https://img.shields.io/badge/learn%20by-examples-0C8EC5.svg?style=for-the-badge&logo=go)](https://github.com/pefish/anywherectl)

Anywherectl is a tool to remote control anything. Enjoy it !!!

## Quick start

### Start Server

```shell script
anywherectl serve -tcp-address=0.0.0.0:8181 -listener-token=test_token -client-token=test -log-level=debug
```

### Start listener

```shell script
anywherectl listen -name=pefish -server-token=test_token -server-address=0.0.0.0:8181 -config=/path/to/config.yaml
```

### Exec "ls" shell

```shell script
anywherectl -listener-name=pefish -listener-token=token_test -server-token=test_token -server-address=0.0.0.0:8181 -action=shell -data=ls
```

## Document

[doc](https://godoc.org/github.com/pefish/anywherectl)

## Todo and Done

- [x] REGISTER command (listener -> server)
- [x] REGISTER_OK command (server -> listener)
- [x] REGISTER_FAIL command (server -> listener)
- [x] Heartbeat between server and listeners (server与listener之间的心跳机制)
- [ ] Listener reconnection (listener重连server机制)
- [ ] SHELL command (server -> listener)
- [ ] RESPONSE_SHELL (listener -> server)

## Security Vulnerabilities

If you discover a security vulnerability, please send an e-mail to [pefish@qq.com](mailto:pefish@qq.com). All security vulnerabilities will be promptly addressed.

## License

This project is licensed under the [Apache License](LICENSE), just like the Go project itself.



