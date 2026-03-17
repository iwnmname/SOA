.PHONY: proto install-tools

install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	mkdir -p gen/flight
	protoc \
		--go_out=. --go_opt=module=flight-booking \
		--go-grpc_out=. --go-grpc_opt=module=flight-booking \
		proto/flight/flight.proto