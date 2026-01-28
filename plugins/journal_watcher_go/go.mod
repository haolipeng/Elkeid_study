module journal_watcher

go 1.18

require (
	github.com/bytedance/plugins v0.0.0
	go.uber.org/zap v1.21.0
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)

replace github.com/bytedance/plugins => ../lib/go
