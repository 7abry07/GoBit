package torrent_test

import (
	"GoBit/internal/torrent"
	"fmt"
	"testing"
	"time"
)

func TestValidTorrent(t *testing.T) {
	file, err := torrent.ParseFile("test_files/naruto.torrent")
	if err != nil {
		t.Errorf("error -> %v", err)
		return
	}

	if file.Announce != "http://nyaa.tracker.wf:7777/announce" {
		t.Errorf("'announce' expected: [%v], | got: [%v]", "nyaa.tracker.wf:7777/announce", file.Announce)
	}

	if file.Name != "[Sotark] Naruto Shippuden [480p][720p][HEVC][x265][Dual-Audio]" {
		t.Errorf("'name' expected: [%v], | got: [%v]", "[Sotark] Naruto Shippuden [480p][720p][HEVC][x265][Dual-Audio]", file.Name)
	}

	if *file.Comment != "https://nyaa.si/view/1189228" {
		t.Errorf("'comment' expected: [%v], | got: [%v]", "https://nyaa.si/view/1189228", *file.Comment)
	}

	if *file.CreatedBy != "NyaaV2" {
		t.Errorf("'created by' expected: [%v], | got: [%v]", "NyaaV213", *file.CreatedBy)
	}

	if *file.Encoding != "utf-8" {
		t.Errorf("'encoding' expected: [%v], | got: [%v]", "utf-8", *file.Encoding)
	}

	if file.CreationDate.Unix() != 1572411720 {
		t.Errorf("'creation date' expected: [%v], | got: [%v]", time.Unix(1572411720, 0).String(), file.CreationDate.String())
	}

	if file.PieceLength != 4194304 {
		t.Errorf("'piece length' expected: [%v], | got: [%v]", 4194304, file.PieceLength)
	}

	if file.Private != false {
		t.Errorf("'private' expected: [%v], | got: [%v]", false, file.Private)
	}

	if len(file.Pieces)%20 != 0 {
		t.Errorf("'pieces' expected: [%v], | got: [%v]", "divisible by 20", "not divsible by 20")
	}

	if file.InfoHash != [20]byte{
		'\xde', '\x2f', '\xee', '\x7c', '\xd8', '\xf3', '\x25', '\x14', '\xdc', '\x13',
		'\x8b', '\x4c', '\xdd', '\x53', '\xc9', '\x3d', '\x7d', '\x7a', '\x1e', '\xb6'} {
		t.Errorf("'info hash' expected: [%v], | got: [%x]", "de2fee7cd8f32514dc138b4cdd53c93d7d7a1eb6", file.InfoHash)
	}

	if len(*file.Files) != 500 {
		t.Errorf("'files' expected: [%v], | got: [%v]", "length == 500", fmt.Sprintf("length == %v", len(*file.Files)))
	}

	if (*file.Files)[0].Path != "[Sotark] Naruto Shippuden - 175 [720p][HEVC][x265][Dual-Audio].mkv" {
		t.Errorf("'files' expected: [%v]Path[%v], | got: [%v]", 0, "[Sotark] Naruto Shippuden - 175 [720p][HEVC][x265][Dual-Audio].mkv", (*file.Files)[0].Path)
	}

	if (*file.Files)[0].Length != 288245151 {
		t.Errorf("'files' expected: [%v]Length[%v], | got: [%v]", 0, 288245151, (*file.Files)[0].Length)
	}

	if len(*file.AnnounceList) != 5 {
		t.Errorf("'announce list' expected: [%v], | got: [%v]", 1, len(*file.AnnounceList))
	}

	if (*file.AnnounceList)[0][0] != "http://nyaa.tracker.wf:7777/announce" {
		t.Errorf("'announce list' expected: [%v][%v], | got: [%v]", 0, "http://nyaa.tracker.wf:7777/announce", (*file.AnnounceList)[0][0])
	}
}
