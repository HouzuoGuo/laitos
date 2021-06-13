all:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o laitos.amd64
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -o laitos.arm64
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -a -o laitos.arm32
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -o laitos.exe
	env CGO_ENABLED=0 GOOS=darwin go build -a -o laitos.darwin

clean:
	rm -f laitos.amd64 laitos.arm64 laitos.arm32 laitos.exe laitos.darwin
