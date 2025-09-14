module battle

go 1.23.2

require (
	github.com/garyburd/redigo v1.6.4
	google.golang.org/grpc v1.72.2
	google.golang.org/protobuf v1.36.5
)

replace common v0.0.0 => ../../common
replace proto v0.0.0 => ../../proto
replace cfg v0.0.0 => ../../cfg_parse
