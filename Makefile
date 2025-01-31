BINDIR:=_output/bin

all: bins pkg/agent/proto/agent.pb.go

bins:
	mkdir -p $(BINDIR)
	go build yunion.io/x/sdnagent/pkg/agent
	go build -o $(BINDIR)/sdnagent yunion.io/x/sdnagent/cmd/sdnagent
	go build -o $(BINDIR)/sdncli yunion.io/x/sdnagent/cmd/sdncli

pkg/agent/proto/agent.pb.go: pkg/agent/proto/agent.proto
	protoc -I pkg/agent/proto pkg/agent/proto/agent.proto --go_out=plugins=grpc:pkg/agent/proto

pkg/agent/proto/agent_pb2.py: pkg/agent/proto/agent.proto
	python -m grpc_tools.protoc -Ipkg/agent/proto --python_out=pkg/agent/proto --grpc_python_out=pkg/agent/proto pkg/agent/proto/agent.proto

test:
	go list ./... | while read p; do \
		go test -v "$$p" ; \
	done

rpm: bins
	EXTRA_BINS=sdncli \
		 $(CURDIR)/build/build.sh sdnagent

.PHONY: all bins rpm test
