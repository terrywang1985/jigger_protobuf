#!/bin/bash

echo "重新生成protobuf代码..."

# 进入proto目录
cd "$(dirname "$0")/proto"

# 检查protoc是否安装
if ! command -v protoc &> /dev/null; then
    echo "错误: protoc 未安装或不在PATH中"
    echo "请安装protobuf编译器: https://grpc.io/docs/protoc-installation/"
    exit 1
fi

# 创建输出目录
mkdir -p ../server/src/proto

echo "生成Go代码..."
# 生成Go代码
protoc --go_out=../server/src/proto \
       --go_opt=paths=source_relative \
       --go-grpc_out=../server/src/proto \
       --go-grpc_opt=paths=source_relative \
       *.proto

echo "生成Python代码..."
# 生成Python代码
protoc --python_out=../client \
       *.proto

echo "protobuf代码生成完成！"

echo ""
echo "注意事项："
echo "1. 请确保生成的Go文件的package声明正确"
echo "2. 如果有导入问题，请检查go.mod文件"
echo "3. Python客户端可以使用生成的*_pb2.py文件"