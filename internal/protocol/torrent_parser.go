package protocol

import (
	"GoBit/internal/bencode"
	"crypto/sha1"
	"net/url"
	"os"
	"strings"
	"time"
)

type fileMode int

const (
	single fileMode = iota
	multi
)

type fileinfo struct {
	Length uint64
	Path   string
}

type TorrentFile struct {
	Announce    *url.URL
	Name        string
	Pieces      []byte
	InfoHash    [20]byte
	PieceLength uint32
	BlockLength uint32
	Private     bool

	AnnounceList *[][]url.URL
	Comment      *string
	CreatedBy    *string
	Encoding     *string
	CreationDate *time.Time
	Files        *[]fileinfo
	Length       *uint64
}

func (f TorrentFile) FileMode() fileMode {
	if f.Files == nil {
		return single
	}
	return multi
}

func ParseTorrentFile(path string) (TorrentFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return TorrentFile{}, err
	}
	file, err := parse(string(content))
	if err != nil {
		return TorrentFile{}, err
	}
	return file, nil
}

func parse(input string) (TorrentFile, error) {
	decoded, err := bencode.Decode(input)
	if err != nil {
		return TorrentFile{}, err
	}
	var f TorrentFile

	root, ok := decoded.Dict()
	if !ok {
		return TorrentFile{}, Torrent_root_not_dict_err
	}

	info, ok := root.FindDict("info")
	if !ok {
		return TorrentFile{}, Torrent_missing_info_err
	}

	announceVal, ok := root.FindStr("announce")
	announce_list, announcelistOk := parseAnnounceList(root)
	if !ok && !announcelistOk {
		return TorrentFile{}, Torrent_missing_announce_err
	}
	announce, err := url.Parse(string(announceVal))
	if err != nil {
		return TorrentFile{}, Torrent_invalid_announce_err
	}

	name, ok := info.FindStr("name")
	if !ok {
		return TorrentFile{}, Torrent_missing_name_err
	}
	pieces, ok := info.FindStr("pieces")
	if !ok {
		return TorrentFile{}, Torrent_missing_pieces_err
	}
	pieceLen, ok := info.FindInt("piece length")
	if !ok {
		return TorrentFile{}, Torrent_missing_piecelen_err
	}

	comment, commentOk := root.FindStr("comment")
	created_by, createdbyOk := root.FindStr("created by")
	encoding, encodingOk := root.FindStr("encoding")
	creation_date, creationdateOk := root.FindInt("creation date")
	private, privateOk := root.FindInt("private")

	length, lengthOk := info.FindInt("length")
	files, filesresOk := parseFiles(info)
	if lengthOk && filesresOk {
		return TorrentFile{}, Torrent_both_length_files_present_err
	}
	if !lengthOk && !filesresOk {
		return TorrentFile{}, Torrent_both_length_files_missing_err
	}

	f.Announce = announce
	f.Name = string(name)
	f.Pieces = []byte(pieces)
	f.PieceLength = uint32(pieceLen)
	f.BlockLength = 16 * 1024
	f.Name = string(name)

	if announcelistOk {
		f.AnnounceList = &announce_list
	}
	if commentOk {
		str := string(comment)
		f.Comment = &str
	}
	if createdbyOk {
		str := string(created_by)
		f.CreatedBy = &str
	}
	if encodingOk {
		str := string(encoding)
		f.Encoding = &str
	}
	if creationdateOk {
		val := int(creation_date)
		date := time.Unix(int64(val), 0)
		f.CreationDate = &date

	}
	if lengthOk {
		val := uint64(length)
		f.Length = &val
	} else {
		f.Files = &files
	}
	if privateOk {
		switch private {
		case 1:
			f.Private = true
		case 0:
			fallthrough
		default:
			f.Private = false
		}
	}
	infoBytes := []byte(bencode.Encode(bencode.NewDict(info)))
	f.InfoHash = sha1.Sum(infoBytes)

	return f, nil
}

func parseAnnounceList(info bencode.BDict) ([][]url.URL, bool) {
	result := [][]url.URL{}
	announceList, ok := info.FindList("announce-list")
	if !ok {
		return [][]url.URL{}, false
	}
	for _, lstnode := range announceList {
		lst, ok := lstnode.List()
		if !ok {
			return [][]url.URL{}, false
		}
		resultLst := []url.URL{}
		for _, strnode := range lst {
			str, ok := strnode.Str()
			if !ok {
				return [][]url.URL{}, false
			}
			ann, err := url.Parse(string(str))
			if err != nil {
				return [][]url.URL{}, false
			}
			resultLst = append(resultLst, *ann)
		}
		result = append(result, resultLst)
	}
	return result, true
}

func parseFiles(info bencode.BDict) ([]fileinfo, bool) {
	result := []fileinfo{}
	files, ok := info.FindList("files")
	if !ok {
		return []fileinfo{}, false
	}
	for _, filenode := range files {
		filedict, ok := filenode.Dict()
		if !ok {
			return []fileinfo{}, false
		}
		file, ok := parseFilesItem(filedict)
		if !ok {
			return []fileinfo{}, false
		}
		result = append(result, file)
	}
	return result, true
}

func parseFilesItem(fileval bencode.BDict) (fileinfo, bool) {
	length, lOk := fileval.FindInt("length")
	pathlst, pOk := fileval.FindList("path")
	if !lOk || !pOk {
		return fileinfo{}, false
	}

	var path strings.Builder
	for _, frag := range pathlst {
		strval, ok := frag.Str()
		if !ok {
			return fileinfo{}, false
		}
		path.WriteString(string(strval))
	}

	return fileinfo{Path: path.String(), Length: uint64(length)}, true
}
