package protocol_test

import (
	"GoBit/internal/protocol"
	"testing"
	"time"
)

func TestValidTorrent(t *testing.T) {
	file, err := protocol.ParseTorrentFile("test_files/debian.torrent")
	if err != nil {
		t.Errorf("error -> %v", err)
		return
	}

	if file.Announce.String() != "http://bttracker.debian.org:6969/announce" {
		t.Errorf("'announce' expected: [%v], | got: [%v]", "http://bttracker.debian.org:6969/announce", file.Announce.String())
	}

	if file.Name != "debian-13.4.0-amd64-netinst.iso" {
		t.Errorf("'name' expected: [%v], | got: [%v]", "debian-13.4.0-amd64-netinst.iso", file.Name)
	}

	if *file.Comment != "Debian CD from cdimage.debian.org" {
		t.Errorf("'comment' expected: [%v], | got: [%v]", "Debian CD from cdimage.debian.org", *file.Comment)
	}

	if *file.CreatedBy != "mktorrent 1.1" {
		t.Errorf("'created by' expected: [%v], | got: [%v]", "mktorrent 1.1", *file.CreatedBy)
	}

	if file.CreationDate.Unix() != 1773496473 {
		t.Errorf("'creation date' expected: [%v], | got: [%v]", time.Unix(1773496473, 0).String(), file.CreationDate.String())
	}

	if file.PieceSize != 262144 {
		t.Errorf("'piece length' expected: [%v], | got: [%v]", 262144, file.PieceSize)
	}

	if file.Private != false {
		t.Errorf("'private' expected: [%v], | got: [%v]", false, file.Private)
	}

	if len(file.Pieces)%20 != 0 {
		t.Errorf("'pieces' expected: [%v], | got: [%v]", "divisible by 20", "not divsible by 20")
	}
}
