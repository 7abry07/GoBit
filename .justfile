alias r := run
alias b := build
alias d := debug
alias t := test
alias ta := testall

@run:
    go run cmd/app/main.go

@build:
    go build cmd/app/main.go

@debug:
    gdlv run cmd/app/main.go

@test target flags="":
    go test {{ flags }} internal/tests/{{ target }}/{{ target }}_test.go

@testall flags="":
    echo "BENCODE TEST SUITE"
    echo "--------------------------"
    just test bencode {{ flags }}
    echo "--------------------------"
    echo ""
    echo "TORRENT TEST SUITE"
    echo "--------------------------"
    just test torrent {{ flags }}
    echo "--------------------------"
