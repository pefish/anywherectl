
```shell script
go run --race ./bin/anywherectl/ serve --config=/path/to/config/file

go run --race ./bin/anywherectl/ listen --config=/path/to/config/file

go run --race ./bin/anywherectl/ --config=/path/to/config/file --action=shell --data="kubectl get pods"

go tool pprof -http=:8081 ./anywherectl http://localhost:9191/debug/pprof/profile

go tool trace http://localhost:9191/debug/pprof/trace?seconds=20
```
