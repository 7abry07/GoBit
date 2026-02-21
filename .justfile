alias r := run
alias b := build
alias d := debug
alias t := test

@run args="":
    go run cmd/app/main.go {{ args }}

@build:
    go build cmd/app/main.go

@debug:
    dlv debug cmd/app/main.go

@test flags="":
    echo "BENCODE TEST SUITE"
    echo "--------------------------"
    go test {{ flags }} GoBit/internal/tests/bencode/
    echo "--------------------------"
    echo ""
    echo "PROTOCOL TEST SUITE"
    echo "--------------------------"
    go test {{ flags }} GoBit/internal/tests/protocol/
    echo "--------------------------"
