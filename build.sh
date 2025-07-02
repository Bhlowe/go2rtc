#!/bin/sh
set -e

check_command() {
    command -v $1 &> /dev/null || {
        echo "Error: $1 could not be found. Please install it."
        exit 1
    }
}


# Check for required commands
check_command go
check_command 7z
check_command upx

# Windows amd64
export GOOS=windows
export GOARCH=amd64
FILENAME="go2rtc.exe"
BUILD_DATE=$(date '+%m/%d/%Y')
go build -ldflags "-s -w -X 'github.com/AlexxIT/go2rtc/internal/app.BuildDate=$BUILD_DATE'" -trimpath  # && 7z a -mx9 -bso0 -sdel $FILENAME go2rtc.exe
cp $FILENAME /data/dev/siteproxy/siteproxy/bin/win64/go2rtc.exe

# Linux amd64
export GOOS=linux
export GOARCH=amd64
FILENAME="go2rtc_linux_amd64"
go build -ldflags "-s -w -X 'github.com/AlexxIT/go2rtc/internal/app.BuildDate=$BUILD_DATE'" -trimpath -o $FILENAME  && upx --lzma --force-overwrite -q --no-progress $FILENAME
cp $FILENAME /data/dev/siteproxy/siteproxy/bin/linux-amd64/go2rtc
