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

	RootName string

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
	switch j.(type) {
	case DiskWriteJob:
		{
			// fmt.Println("DISK WRITE JOB STARTED")
			j := j.(DiskWriteJob)
			go dm.writeBlock(j.PieceIdx, j.BlockIdx, j.Data)
		}
	case DiskReadJob:
		{
			// fmt.Println("DISK READ JOB STARTED")
			j := j.(DiskReadJob)
			go dm.readBlock(j.RequestedFrom, j.PieceIdx, j.BlockIdx, j.Length)
		}
	case DiskHashJob:
		{
			// fmt.Println("DISK HASH JOB STARTED")
			j := j.(DiskHashJob)
			go dm.verifyHash(j.PieceIdx)
		}
	}
}

func (dm *DiskManager) writeBlock(pieceIdx, blockIdx uint32, data []byte) {
	pieceStart := uint64(pieceIdx) * uint64(dm.PieceSize)
	blockStart := uint64(blockIdx) * uint64(dm.BlockSize)

	dataStart := pieceStart + blockStart
	dataEnd := dataStart + uint64(dm.BlockSize)

	for _, entry := range dm.files {
		fileEnd := entry.Offset + entry.Size
		overlapStart := max(entry.Offset, dataStart)
		overlapEnd := min(fileEnd, dataEnd)

		if overlapStart < overlapEnd {
			fileOffset := overlapStart - entry.Offset
			blockOffset := overlapStart - dataStart
			length := overlapEnd - overlapStart

			fullPath := filepath.Join(dm.RootName, entry.Path)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			if err != nil {
				dm.torrent.SignalEvent(DiskWriteFailedEv{
					pieceIdx, blockIdx, err,
				})
				return
			}

			file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE, 0644)
			defer file.Close()
			if err != nil {
				dm.torrent.SignalEvent(DiskWriteFailedEv{
					pieceIdx, blockIdx, err,
				})
				return
			}

			_, err = file.WriteAt(data[blockOffset:blockOffset+length], int64(fileOffset))
			if err != nil {
				dm.torrent.SignalEvent(DiskWriteFailedEv{
					pieceIdx, blockIdx, err,
				})
				return
			}
			fmt.Printf("WRITTEN [%v:%v] AT OFFSET (%v, %v) IN FILE %v\n", pieceIdx, blockIdx, fileOffset, fileOffset+length, entry.Path)
		}
	}
	dm.torrent.SignalEvent(DiskWriteSuccessfulEv{
		pieceIdx, blockIdx,
	})
}

func (dm *DiskManager) readBlock(requestedFrom PeerID, pieceIdx, blockIdx uint32, length uint32) {
	pieceStart := uint64(pieceIdx) * uint64(dm.PieceSize)
	blockStart := uint64(blockIdx) * uint64(dm.BlockSize)

	dataStart := uint64(pieceStart + blockStart)
	dataEnd := uint64(dataStart + uint64(dm.BlockSize))

	block := []byte{}

	for _, entry := range dm.files {
		fileEnd := entry.Offset + entry.Size
		overlapStart := max(entry.Offset, dataStart)
		overlapEnd := min(fileEnd, dataEnd)

		if overlapStart < overlapEnd {
			fileOffset := overlapStart - entry.Offset
			length := overlapEnd - overlapStart

			fullPath := filepath.Join(dm.RootName, entry.Path)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			if err != nil {
				return
			}

			file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE, 0644)
			defer file.Close()
			if err != nil {
				return
			}

			buf := make([]byte, length)
			_, err = file.ReadAt(buf, int64(fileOffset))
			if err != nil {
				return
			}
			block = append(block, buf...)
		}
	}

	dm.torrent.SignalEvent(DiskReadSuccessfulEv{
		requestedFrom, pieceIdx, blockIdx, block,
	})
}

func (dm *DiskManager) verifyHash(pieceIdx uint32) {
	pieceStart := uint64(pieceIdx) * uint64(dm.PieceSize)
	pieceEnd := pieceStart + uint64(dm.PieceSize)

	actualPieceHash := dm.torrent.Info.Pieces[pieceIdx*20 : (pieceIdx*20)+20]
	piece := []byte{}

	for _, entry := range dm.files {
		fileEnd := entry.Offset + entry.Size
		overlapStart := max(entry.Offset, pieceStart)
		overlapEnd := min(fileEnd, pieceEnd)

		if overlapStart < overlapEnd {
			fileOffset := overlapStart - entry.Offset
			length := overlapEnd - overlapStart

			fullPath := filepath.Join(dm.RootName, entry.Path)
			file, err := os.OpenFile(fullPath, os.O_RDONLY, 0644)
			defer file.Close()
			if err != nil {
				dm.torrent.SignalEvent(DiskHashFailedEv{pieceIdx, err})
				return
			}

			buf := make([]byte, length)
			_, err = file.ReadAt(buf, int64(fileOffset))
			if err != nil && err != io.EOF {
				dm.torrent.SignalEvent(DiskHashFailedEv{pieceIdx, err})
				return
			}

			piece = append(piece, buf...)
			// fmt.Printf("READ [%v] AT OFFSET (%v, %v) IN FILE %v\n", pieceIdx, fileOffset, fileOffset+length, entry.Path)
		}
	}

	if len(piece) != int(dm.PieceSize) {
		dm.torrent.SignalEvent(DiskHashFailedEv{pieceIdx, fmt.Errorf("piece length in hash check doesn't match")})
		return
	}

	hasher := sha1.New()
	hasher.Write(piece)
	pieceHash := hasher.Sum([]byte{})

	if slices.Compare(pieceHash, actualPieceHash) != 0 {
		dm.torrent.SignalEvent(DiskHashFailedEv{pieceIdx, nil})
	} else {
		dm.torrent.SignalEvent(DiskHashSuccessfulEv{pieceIdx})
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

func (dm *DiskManager) GetPieceSize(idx uint32) uint32 {
	if idx != dm.PieceCount-1 {
		return dm.PieceSize
	} else {
		lastFileEnd := dm.files[len(dm.files)-1].Offset + dm.files[len(dm.files)-1].Size
		return dm.PieceSize - ((dm.PieceCount * dm.PieceSize) - uint32(lastFileEnd))
	}
}

func (dm *DiskManager) EnqueueJob(job DiskJob) {
	dm.jobs <- job
}
