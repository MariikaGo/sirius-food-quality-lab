#!/bin/zsh
set -e
cd "${0:A:h:h}"
protoc -Iproto --go_out=. --go_opt=module=sirius.local/food --go-grpc_out=. --go-grpc_opt=module=sirius.local/food proto/delivery.proto
