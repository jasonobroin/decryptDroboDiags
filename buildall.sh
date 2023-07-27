echo Building Linux/x86
env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w"
mv decryptDiags decryptDiags-lx
echo Building Windows
env GOOS=windows GOARCH=386 go build -ldflags "-s -w"
echo Building Mac
env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w"

# Build docker image
#docker build -t decryptdiags .
