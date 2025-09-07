module battle

go 1.23.2

require common v0.0.0

require (
	github.com/garyburd/redigo v1.6.4
	google.golang.org/grpc v1.72.2
	google.golang.org/protobuf v1.36.5
	proto v0.0.0
)

require (
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
)

replace common v0.0.0 => ../../common

replace proto v0.0.0 => ../../proto

replace cfg v0.0.0 => ../../cfg_parse
