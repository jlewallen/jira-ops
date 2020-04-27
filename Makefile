default: all

all: build/jira-status

build/jira-status: *.go
	mkdir -p build
	go build -o build/jira-status *.go

clean:
	rm -rf build
