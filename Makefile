run: maglev0
	sudo ./maglev0

maglev0: main.go
	go build .

test:
	go test -v

deps:
	go get .
