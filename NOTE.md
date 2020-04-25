
### 本地启动

```shell script
go run --race ./bin/main/ serve --listener-token=test_token --client-token=test --log-level=debug

go run --race ./bin/main/ listen --server-token=test_token --server-address=0.0.0.0:8181 --name=pefish --log-level=debug --config=/path/to/config/file

go run --race ./bin/main/ --listener-name=pefish --listener-token=token_test --server-token=test --server-address=0.0.0.0:8181 --action=shell --data=ls --log-level=debug

go tool pprof -http=:8081 ./main http://localhost:9191/debug/pprof/profile
```
