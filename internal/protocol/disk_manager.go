package protocol

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
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

	PieceCount    uint32
	PieceSize     uint32
	BlockSize     uint32
	BlockPerPiece uint32

	LastPieceSize     uint32
	LastBlockPerPiece uint32

	jobs chan DiskJob

	files []FileEntry
}

func NewDiskManager(torrent *Torrent, totalSize uint64, pieceCount, pieceSize, blockSize uint32) *DiskManager {
	dm := DiskManager{}

	dm.ctx, dm.cancel = context.WithCancel(torrent.ctx)
	dm.torrent = torrent
	dm.RootName = ""
	dm.PieceCount = pieceCount
	dm.PieceSize = pieceSize
	dm.BlockSize = blockSize
	dm.BlockPerPiece = pieceSize / blockSize
	dm.LastPieceSize = uint32(totalSize % uint64(pieceSize))
	if dm.LastPieceSize == 0 {
		dm.LastPieceSize = pieceSize
	}
	dm.LastBlockPerPiece = dm.LastPieceSize / blockSize
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
				if err != nil {
					dm.torrent.SignalEvent(DiskWriteFailed{j.PieceIdx, j.BlockOff, err})
				} else {
					dm.torrent.SignalEvent(DiskWriteSuccessful{j.PieceIdx, j.BlockOff})
				}
			}()
		}
	case DiskReadJob:
		{
			// fmt.Println("DISK READ JOB STARTED")
			go func() {
				data, err := dm.readBlock(j.RequestedFrom, j.PieceIdx, j.BlockOff, j.Length)
				if err != nil {
					dm.torrent.SignalEvent(DiskReadFailed{j.RequestedFrom, j.PieceIdx, j.BlockOff, err})
				} else {
					dm.torrent.SignalEvent(DiskReadSuccessful{j.RequestedFrom, j.PieceIdx, j.BlockOff, data})
				}
			}()
		}
	case DiskHashJob:
		{
			// fmt.Println("DISK HASH JOB STARTED")
			go func() {
				err := dm.verifyHash(j.PieceIdx)
				if err != nil {
					dm.torrent.SignalEvent(DiskHashFailed{j.PieceIdx, err})
				} else {
					dm.torrent.SignalEvent(DiskHashPassed{j.PieceIdx})
				}
			}()
		}
	}
}

func (dm *DiskManager) writeBlock(pieceIdx, blockOff uint32, data []byte) error {
	pieceStart := uint64(pieceIdx) * uint64(dm.PieceSize)

	dataStart := pieceStart + uint64(blockOff)
	dataEnd := dataStart + uint64(dm.GetBlockSize(pieceIdx, blockOff))

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

			if pieceIdx == 11217 {
				fmt.Printf("WRITTEN [%v:%v] AT OFFSET (%v : %v) IN FILE %v\n", pieceIdx, blockOff, fileOffset, length, entry.Path)
			}
		}
	}
	return nil
}

func (dm *DiskManager) readBlock(requestedFrom PeerID, pieceIdx, blockOff uint32, length uint32) ([]byte, error) {
	pieceStart := uint64(pieceIdx) * uint64(dm.PieceSize)

	dataStart := uint64(pieceStart + uint64(blockOff))
	dataEnd := dataStart + uint64(dm.GetBlockSize(pieceIdx, blockOff))

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
	pieceStart := uint64(pieceIdx) * uint64(dm.PieceSize)
	pieceEnd := pieceStart + uint64(dm.GetPieceSize(pieceIdx))

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
			if pieceIdx == 11217 {
				fmt.Printf("READ [%v] AT OFFSET (%v : %v) IN FILE %v\n", pieceIdx, fileOffset, length, entry.Path)
			}
		}
	}

	if len(piece) != int(dm.GetPieceSize(pieceIdx)) {
		return fmt.Errorf("piece length in hash check doesn't match")
	}

	hasher := sha1.New()
	hasher.Write(piece)
	pieceHash := hasher.Sum([]byte{})

	if slices.Compare(pieceHash, actualPieceHash) != 0 {
		fmt.Printf("right: %v\n", actualPieceHash)
		fmt.Printf("found: %v\n", pieceHash)
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

func (dm *DiskManager) GetPieceSize(idx uint32) uint32 {
	if idx != dm.PieceCount-1 {
		return dm.PieceSize
	} else {
		lastFileEnd := dm.files[len(dm.files)-1].Offset + dm.files[len(dm.files)-1].Size
		return dm.PieceSize - ((dm.PieceCount * dm.PieceSize) - uint32(lastFileEnd))
	}
}

func (dm *DiskManager) GetBlockSize(pieceIdx, blockOff uint32) uint32 {
	remaining := dm.GetPieceSize(pieceIdx) - blockOff
	return min(remaining, dm.BlockSize)
}

func (dm *DiskManager) EnqueueJob(job DiskJob) {
	dm.jobs <- job
}
