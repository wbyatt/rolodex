package server

import (
	"encoding/binary"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

type LogEntry struct {
	timestamp int64
	key       string
	value     string
}

type Wal struct {
	buffer   []LogEntry
	syncLock sync.Mutex
	file     *os.File
	fileName string
}

func BuildWal(fileName string, memstore *map[string]string) *Wal {
	if _, err := os.Stat(fileName); err == nil {
		// open the fileName and read the first 64 bytes
		file, err := os.OpenFile(fileName, os.O_RDONLY, 0644)

		if err != nil {
			panic(err)
		}

		defer file.Close()

		for {
			var timestamp int64
			var keyLen uint16
			var valueLen uint32

			if err = binary.Read(file, binary.LittleEndian, &timestamp); err != nil {
				if err == io.EOF {
					break
				}

				panic(err)
			}
			log.Printf("Reconstructed timestamp: %d", timestamp)

			if err = binary.Read(file, binary.LittleEndian, &keyLen); err != nil {
				if err == io.EOF {
					break
				}

				panic(err)
			}

			log.Printf("Reconstructed key length: %d", keyLen)

			key := make([]byte, keyLen)

			if err = binary.Read(file, binary.LittleEndian, &key); err != nil {
				if err == io.EOF {
					break
				}

				panic(err)
			}

			log.Printf("Reconstructed key: %s", key)

			if err = binary.Read(file, binary.LittleEndian, &valueLen); err != nil {
				if err == io.EOF {
					break
				}

				panic(err)
			}

			value := make([]byte, valueLen)

			if err = binary.Read(file, binary.LittleEndian, &value); err != nil {
				if err == io.EOF {
					break
				}

				panic(err)
			}

			log.Printf("Reconstructed value: %s", value)

			(*memstore)[string(key)] = string(value)
		}

	}

	return NewWal(fileName)
}

func NewWal(fileName string) *Wal {
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		panic(err)
	}

	wal := &Wal{
		buffer:   make([]LogEntry, 0),
		file:     file,
		fileName: fileName,
	}

	go func() {

		for {
			if len(wal.buffer) == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			wal.syncLock.Lock()

			for _, entry := range wal.buffer {
				keyBytes := []byte(entry.key)
				valueBytes := []byte(entry.value)

				binary.Write(wal.file, binary.LittleEndian, entry.timestamp)
				binary.Write(wal.file, binary.LittleEndian, uint16(len(keyBytes)))
				binary.Write(wal.file, binary.LittleEndian, keyBytes)
				binary.Write(wal.file, binary.LittleEndian, uint32(len(valueBytes)))
				binary.Write(wal.file, binary.LittleEndian, valueBytes)
			}

			file.Sync()
			wal.buffer = make([]LogEntry, 0)
			wal.syncLock.Unlock()
			time.Sleep(1000 * time.Millisecond)
		}
	}()

	return wal
}

func (wal *Wal) Write(key string, value string) {
	wal.syncLock.Lock()
	defer wal.syncLock.Unlock()

	wal.buffer = append(wal.buffer, LogEntry{
		timestamp: time.Now().Unix(),
		key:       key,
		value:     value,
	})
}
