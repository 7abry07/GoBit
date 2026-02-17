package protocol

import (
	"net"
)

func WriteFull(conn net.Conn, content []byte) error {
	writtenCnt := 0
	for writtenCnt < len(content) {
		written, err := conn.Write(content)
		if err != nil {
			return err
		}
		writtenCnt += written
	}
	return nil
}
