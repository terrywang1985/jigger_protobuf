rem protoc.exe --proto_path=../proto --go_out=../server/src/proto --go_opt=paths=source_relative ../proto/*.proto

cd /d "%~dp0"

protoc.exe ^
  --proto_path=../proto ^
  --go_out=../server/src/proto ^
  --go_opt=paths=source_relative ^
  --go-grpc_out=../server/src/proto ^
  --go-grpc_opt=paths=source_relative ^
  ../proto/*.proto

protoc.exe -I=../proto --python_out=../client/ ../proto/*.proto