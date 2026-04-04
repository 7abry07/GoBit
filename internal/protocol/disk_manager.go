package protocol

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
)

type FileEntry struct {
	Path   string
	Size   uint64
	Offset uint64
}

type DiskManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	torrent *Torrent

	RootName          string
	DownloadDirectory string

	pieceCount    uint32
	pieceSize     uint32
	blockSize     uint32
	blockPerPiece uint32

	lastpieceSize     uint32
	lastBlockPerPiece uint32

	jobs chan DiskJob

	files []FileEntry
}

func NewDiskManager(torrent *Torrent, totalSize uint64, pieceCount, pieceSize, blockSize uint32) *DiskManager {
	dm := DiskManager{}

	dm.ctx, dm.cancel = context.WithCancel(torrent.ctx)
	dm.torrent = torrent
	dm.RootName = ""
	dm.pieceCount = pieceCount
	dm.pieceSize = pieceSize
	dm.blockSize = blockSize
	dm.blockPerPiece = pieceSize / blockSize
	dm.lastpieceSize = uint32(totalSize % uint64(pieceSize))
	if dm.lastpieceSize == 0 {
		dm.lastpieceSize = pieceSize
	}
	dm.lastBlockPerPiece = uint32(math.Ceil(float64(dm.lastpieceSize) / float64(blockSize)))
	dm.files = nil

	dm.jobs = make(chan DiskJob, 1024)

	go dm.loop()

	return &dm
}

func (dm *DiskManager) loop() {
	for {
		select {
		case <-dm.ctx.Done():
			dm.files = nil
			return
		case j := <-dm.jobs:
			dm.startJob(j)
		}
	}
}

func (dm *DiskManager) startJob(j DiskJob) {
	switch j := j.(type) {
	case DiskWriteJob:
		{
			// fmt.Println("DISK WRITE JOB STARTED")
			go func() {
				err := dm.writeBlock(j.PieceIdx, j.BlockOff, j.Data)
				dm.torrent.SignalEvent(DiskWriteFinished{j.PieceIdx, j.BlockOff, err})
			}()
		}
	case DiskReadJob:
		{
			// fmt.Println("DISK READ JOB STARTED")
			go func() {
				data, err := dm.readBlock(j.RequestedFrom, j.PieceIdx, j.BlockOff, j.Length)
				dm.torrent.SignalEvent(DiskReadFinished{j.RequestedFrom, j.PieceIdx, j.BlockOff, data, err})
			}()
		}
	case DiskHashJob:
		{
			// fmt.Println("DISK HASH JOB STARTED")
			go func() {
				err := dm.verifyHash(j.PieceIdx)
				dm.torrent.SignalEvent(DiskHashFinished{j.PieceIdx, err})
			}()
		}
	}
}

func (dm *DiskManager) writeBlock(pieceIdx, blockOff uint32, data []byte) error {
	pieceStart := uint64(pieceIdx) * uint64(dm.pieceSize)

	dataStart := pieceStart + uint64(blockOff)
	dataEnd := dataStart + uint64(dm.GetblockSize(pieceIdx, blockOff))

	for _, entry := range dm.files {
		fileEnd := entry.Offset + entry.Size
		overlapStart := max(entry.Offset, dataStart)
		overlapEnd := min(fileEnd, dataEnd)

		if overlapStart < overlapEnd {
			fileOffset := overlapStart - entry.Offset
			blockOffset := overlapStart - dataStart
			length := overlapEnd - overlapStart

			fullPath := dm.GetFullPath(entry)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			if err != nil {
				return err
			}

			file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				return err
			}

			_, err = file.WriteAt(data[blockOffset:blockOffset+length], int64(fileOffset))
			if err != nil {
				return err
			}

			// fmt.Printf("WRITTEN [%v:%v] AT OFFSET (%v : %v) IN FILE %v\n", pieceIdx, blockOff, fileOffset, length, entry.Path)
		}
	}
	return nil
}

func (dm *DiskManager) readBlock(requestedFrom PeerID, pieceIdx, blockOff uint32, length uint32) ([]byte, error) {
	pieceStart := uint64(pieceIdx) * uint64(dm.pieceSize)

	dataStart := uint64(pieceStart + uint64(blockOff))
	dataEnd := dataStart + uint64(dm.GetblockSize(pieceIdx, blockOff))

	block := []byte{}

	for _, entry := range dm.files {
		fileEnd := entry.Offset + entry.Size
		overlapStart := max(entry.Offset, dataStart)
		overlapEnd := min(fileEnd, dataEnd)

		if overlapStart < overlapEnd {
			fileOffset := overlapStart - entry.Offset
			length := overlapEnd - overlapStart

			fullPath := dm.GetFullPath(entry)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			if err != nil {
				return nil, err
			}

			file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				return nil, err
			}

			buf := make([]byte, length)
			_, err = file.ReadAt(buf, int64(fileOffset))
			if err != nil {
				return nil, err
			}
			block = append(block, buf...)
			fmt.Printf("READ [%v:%v] AT OFFSET (%v : %v) IN FILE %v\n", pieceIdx, blockOff, fileOffset, length, entry.Path)
		}
	}
	return block, nil
}

func (dm *DiskManager) verifyHash(pieceIdx uint32) error {
	pieceStart := uint64(pieceIdx) * uint64(dm.pieceSize)
	pieceEnd := pieceStart + uint64(dm.GetpieceSize(pieceIdx))

	actualPieceHash := dm.torrent.Info.Pieces[pieceIdx*20 : (pieceIdx*20)+20]
	piece := []byte{}

	for _, entry := range dm.files {
		fileEnd := entry.Offset + entry.Size
		overlapStart := max(entry.Offset, pieceStart)
		overlapEnd := min(fileEnd, pieceEnd)

		if overlapStart < overlapEnd {
			fileOffset := overlapStart - entry.Offset
			length := overlapEnd - overlapStart

			fullPath := dm.GetFullPath(entry)
			file, err := os.OpenFile(fullPath, os.O_RDONLY, 0644)
			if err != nil {
				return err
			}

			buf := make([]byte, length)
			_, err = file.ReadAt(buf, int64(fileOffset))
			if err != nil && err != io.EOF {
				return err
			}

			piece = append(piece, buf...)
			// fmt.Printf("READ [%v] AT OFFSET (%v : %v) IN FILE %v\n", pieceIdx, fileOffset, length, entry.Path)
		}
	}

	if len(piece) != int(dm.GetpieceSize(pieceIdx)) {
		return fmt.Errorf("piece length in hash check doesn't match")
	}

	hasher := sha1.New()
	hasher.Write(piece)
	pieceHash := hasher.Sum([]byte{})

	if slices.Compare(pieceHash, actualPieceHash) != 0 {
		// fmt.Printf("right: %v\n", actualPieceHash)
		// fmt.Printf("found: %v\n", pieceHash)
		return fmt.Errorf("hash check failed")
	} else {
		return nil
	}
}

func (dm *DiskManager) AddFile(path string, size uint64) {
	fileEntry := FileEntry{
		Path: path,
		Size: size,
	}

	if len(dm.files) == 0 {
		fileEntry.Offset = 0
	} else {
		fileEntry.Offset = dm.files[len(dm.files)-1].Offset + dm.files[len(dm.files)-1].Size
	}

	dm.files = append(dm.files, fileEntry)
}

func (dm *DiskManager) GetFullPath(file FileEntry) string {
	if len(dm.files) == 1 {
		return filepath.Join(dm.DownloadDirectory, file.Path)
	} else {
		return filepath.Join(dm.DownloadDirectory, dm.RootName, file.Path)
	}
}

func (dm *DiskManager) GetpieceSize(idx uint32) uint32 {
	if idx != dm.pieceCount-1 {
		return dm.pieceSize
	} else {
		lastFileEnd := dm.files[len(dm.files)-1].Offset + dm.files[len(dm.files)-1].Size
		return dm.pieceSize - ((dm.pieceCount * dm.pieceSize) - uint32(lastFileEnd))
	}
}

func (dm *DiskManager) GetblockSize(pieceIdx, blockOff uint32) uint32 {
	remaining := dm.GetpieceSize(pieceIdx) - blockOff
	return min(remaining, dm.blockSize)
}

func (dm *DiskManager) EnqueueJob(job DiskJob) {
	dm.jobs <- job
}
