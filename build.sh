GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildmode=c-shared \
                                               -o                  \
                                               libtg.so            \
                                               -trimpath           \
                                               -buildvcs=false     \
                                               -ldflags="-s -w -buildid="
