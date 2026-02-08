alias r := run
alias b := build
alias d := debug

@run:
    go run cmd/app/main.go

@build:
    go build cmd/app/main.go

@debug:
    gdlv run cmd/app/main.go
