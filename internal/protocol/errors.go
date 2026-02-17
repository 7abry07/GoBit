package protocol

import "errors"

var (
	Root_not_dict_err    error = errors.New("the root structure is not a dictionary")
	Missing_announce_err error = errors.New("the mandatory field 'announce' is missing")
	Missing_info_err     error = errors.New("the mandatory field 'info' is missing")
	Missing_name_err     error = errors.New("the mandatory field 'key' is missing")
	Missing_pieces_err   error = errors.New("the mandatory field 'pieces' is missing")
	Missing_piecelen_err error = errors.New("the mandatory field 'piece length' is missing")

	Invalid_announce_err error = errors.New("the 'announce' field is invalid")
	Invalid_name_err     error = errors.New("the 'name' field is invalid")
	Invalid_pieces_err   error = errors.New("the 'pieces' field is invalid")
	Invalid_piecelen_err error = errors.New("the 'piece length' field is invalid")
	Invalid_length_err   error = errors.New("the 'length' field is invalid")
	Invalid_files_err    error = errors.New("the 'files' field is invalid")

	Both_length_files_present_err error = errors.New("both files and length fields are present")
	Both_length_files_missing_err error = errors.New("both files and length fields are missing")

	Bad_peer_handshake_err error = errors.New("the handshake request or response of the peer is invalid")
	Bad_peer_message_err   error = errors.New("the message sent to the peer is invalid")
)
