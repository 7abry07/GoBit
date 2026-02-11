package tracker

import "errors"

var (
	invalid_scheme_err       = errors.New("the url scheme is neither 'http' nor 'udp'")
	invalid_tracker_resp_err = errors.New("the tracker response is in an invalid format")
	scrape_not_supported_err = errors.New("the tracker does not support scrape requests")
)
