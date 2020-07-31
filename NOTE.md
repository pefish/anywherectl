
```shell script
go run --race ./cmd/anywherectl/ serve --config=/path/to/config/file

go run --race ./cmd/anywherectl/ listen --config=/path/to/config/file

go run --race ./cmd/anywherectl/ --config=/path/to/config/file --action=shell --data="kubectl get pods"

go tool pprof -http=:8081 ./anywherectl http://localhost:9191/debug/pprof/profile?seconds=120

go tool trace http://localhost:9191/debug/pprof/trace
```
