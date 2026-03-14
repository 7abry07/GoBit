package protocol

import "errors"

var (
	Torrent_root_not_dict_err             error = errors.New("the root structure is not a dictionary")
	Torrent_missing_announce_err          error = errors.New("the mandatory field 'announce' is missing")
	Torrent_missing_info_err              error = errors.New("the mandatory field 'info' is missing")
	Torrent_missing_name_err              error = errors.New("the mandatory field 'key' is missing")
	Torrent_missing_pieces_err            error = errors.New("the mandatory field 'pieces' is missing")
	Torrent_missing_piecelen_err          error = errors.New("the mandatory field 'piece length' is missing")
	Torrent_invalid_announce_err          error = errors.New("the 'announce' field is invalid")
	Torrent_invalid_name_err              error = errors.New("the 'name' field is invalid")
	Torrent_invalid_pieces_err            error = errors.New("the 'pieces' field is invalid")
	Torrent_invalid_piecelen_err          error = errors.New("the 'piece length' field is invalid")
	Torrent_invalid_length_err            error = errors.New("the 'length' field is invalid")
	Torrent_invalid_files_err             error = errors.New("the 'files' field is invalid")
	Torrent_both_length_files_present_err error = errors.New("both files and length fields are present")
	Torrent_both_length_files_missing_err error = errors.New("both files and length fields are missing")

	Tracker_invalid_url_err          error = errors.New("the tracker url is invalid")
	Tracker_invalid_scheme_err       error = errors.New("the tracker url scheme is neither 'http' nor 'udp'")
	Tracker_invalid_resp_err         error = errors.New("the tracker response is in an invalid format")
	Tracker_scrape_not_supported_err error = errors.New("the tracker does not support scrape requests")

	Peer_bad_handshake_err error = errors.New("the handshake request or response of the peer is invalid")
	Peer_bad_message_err   error = errors.New("the message sent to the peer is invalid")
	Peer_id_mismatch_err   error = errors.New("the message sent to the peer is invalid")
)
