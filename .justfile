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

@test target:
    go test internal/tests/{{ target }}/{{ target }}_test.go

@testall:
    just test bencode
