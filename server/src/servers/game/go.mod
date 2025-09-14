module game

go 1.23.2

require (
	github.com/garyburd/redigo v1.6.4
	github.com/gorilla/websocket v1.5.3
	google.golang.org/grpc v1.72.2
	google.golang.org/protobuf v1.36.5
)

require cfg v0.0.0 // indirect

replace common v0.0.0 => ../../common

replace proto v0.0.0 => ../../proto

replace cfg v0.0.0 => ../../cfg_parse
