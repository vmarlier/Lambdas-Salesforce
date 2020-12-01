all: zip

compile:
	GOOS=linux GOARCH=amd64 go build main.go

zip: compile
	zip main.zip main
