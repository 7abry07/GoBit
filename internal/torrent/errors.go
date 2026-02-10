package torrent

import "errors"

var (
	root_not_dict_err    error = errors.New("the root structure is not a dictionary")
	missing_announce_err error = errors.New("the mandatory field 'announce' is missing")
	missing_info_err     error = errors.New("the mandatory field 'info' is missing")
	missing_name_err     error = errors.New("the mandatory field 'key' is missing")
	missing_pieces_err   error = errors.New("the mandatory field 'pieces' is missing")
	missing_piecelen_err error = errors.New("the mandatory field 'piece length' is missing")

	invalid_announce_err error = errors.New("the 'announce' field is invalid")
	invalid_name_err     error = errors.New("the 'name' field is invalid")
	invalid_pieces_err   error = errors.New("the 'pieces' field is invalid")
	invalid_piecelen_err error = errors.New("the 'piece length' field is invalid")
	invalid_length_err   error = errors.New("the 'length' field is invalid")
	invalid_files_err    error = errors.New("the 'files' field is invalid")

	both_length_files_present_err error = errors.New("both files and length fields are present")
	both_length_files_missing_err error = errors.New("both files and length fields are missing")
)
