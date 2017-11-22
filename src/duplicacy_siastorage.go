// Free for all

package duplicacy

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/tv42/zbase32"

	"github.com/NebulousLabs/Sia/api"
)

type apiClient interface {
	Get(string, interface{}) error
	Post(string, string, interface{}) error
}

// SIAStorage is a storage backend for Sia.
type SIAStorage struct {
	StorageBase

	storageDir string
	client     apiClient
}

// CreateSIAStorage creates a Sia storage object.
func CreateSIAStorage(storageDir string, client apiClient) (storage *SIAStorage, err error) {
	var contracts api.RenterContracts
	err = client.Get("/renter/contracts", &contracts)
	if err != nil {
		return nil, err
	}
	if len(contracts.Contracts) == 0 {
		return nil, errors.New("You must have formed contracts to upload to Sia")
	}

	storage = &SIAStorage{
		storageDir: storageDir,
		client:     client,
	}

	storage.SetDefaultNestingLevels([]int{0}, 0)
	return storage, nil
}

// ListFiles return the list of files and subdirectories under 'dir' (non-recursively)
func (storage *SIAStorage) ListFiles(threadIndex int, dir string) (files []string, sizes []int64, err error) {
	if len(dir) > 0 && dir[len(dir)-1] != '/' {
		dir += "/"
	}
	dirPath := storage.storageDir + "/" + dir
	dirLength := len(dirPath)

	var rf api.RenterFiles
	err = storage.client.Get("/renter/files", &rf)
	if err != nil {
		return nil, nil, err
	}
	if dir == "snapshots/" {
		for _, fi := range rf.Files {
			if len(fi.SiaPath) > dirLength {
				if fi.SiaPath[:dirLength] == dirPath {
					paths := strings.Split(fi.SiaPath[dirLength:], "/")
					files = append(files, paths[0]+"/")
				}
			}
		}
		return files, nil, nil
	} else if dir == "chunks/" {
		for _, fi := range rf.Files {
			if len(fi.SiaPath) > dirLength {
				if fi.SiaPath[:dirLength] == dirPath {
					files = append(files, fi.SiaPath[dirLength:])
					sizes = append(sizes, int64(fi.Filesize))
				}
			}
		}
		return files, sizes, nil
	} else {
		for _, fi := range rf.Files {
			if len(fi.SiaPath) > dirLength {
				if fi.SiaPath[:dirLength] == dirPath {
					files = append(files, fi.SiaPath[dirLength:])
				}
			}
		}
		return files, nil, nil
	}
}

// DeleteFile deletes the file or directory at 'filePath'.
func (storage *SIAStorage) DeleteFile(threadIndex int, filePath string) (err error) {
	fPath := storage.storageDir + "/" + filePath
	return storage.client.Post(fmt.Sprintf("/renter/delete/%v", fPath), "", nil)
}

// MoveFile renames the file.
func (storage *SIAStorage) MoveFile(threadIndex int, from string, to string) (err error) {
	fromPath := storage.storageDir + "/" + from
	toPath := storage.storageDir + "/" + to
	return storage.client.Post(fmt.Sprintf("/renter/rename/%v", fromPath), fmt.Sprintf("newsiapath=%v", toPath), nil)
}

// CreateDirectory creates a new directory.
func (storage *SIAStorage) CreateDirectory(threadIndex int, dir string) (err error) {
	return nil
}

// GetFileInfo returns the information about the file or directory at 'filePath'.
func (storage *SIAStorage) GetFileInfo(threadIndex int, filePath string) (exist bool, isDir bool, size int64, err error) {
	var rf api.RenterFiles
	err = storage.client.Get("/renter/files", &rf)
	if err != nil {
		return false, false, 0, err
	}

	fPath := storage.storageDir + "/" + filePath
	for _, fi := range rf.Files {
		if fi.SiaPath == fPath {
			return true, false, int64(fi.Filesize), nil
		}
	}
	return false, false, 0, nil
}

// FindChunk finds the chunk with the specified id.  If 'isFossil' is true, it will search for chunk files with
// the suffix '.fsl'.
func (storage *SIAStorage) FindChunk(threadIndex int, chunkID string, isFossil bool) (filePath string, exist bool, size int64, err error) {
	filePath = "chunks/" + chunkID
	if isFossil {
		filePath += ".fsl"
	}

	exist, _, size, err = storage.GetFileInfo(threadIndex, filePath)

	if err != nil {
		return "", false, 0, err
	}
	return filePath, exist, size, err
}

// DownloadFile reads the file at 'filePath' into the chunk.
func (storage *SIAStorage) DownloadFile(threadIndex int, filePath string, chunk *Chunk) (err error) {
	downloadPath := storage.storageDir + "/" + filePath

	fName := make([]byte, 32)
	_, err = rand.Read(fName)
	if err != nil {
		return err
	}
	h := sha256.New()
	h.Write(fName)
	tmpFileName := zbase32.EncodeToString(h.Sum(nil))
	tmpFile, err := ioutil.TempFile(os.TempDir(), tmpFileName)
	err = storage.client.Get(fmt.Sprintf("/renter/download/%v?destination=%v", downloadPath, tmpFile.Name()), nil)
	if err != nil {
		return err
	}

	time.Sleep(time.Second) // give download time to initialize
	for {
		var queue api.RenterDownloadQueue
		err := storage.client.Get("/renter/downloads", &queue)
		if err != nil {
			return err
		}
		var d api.DownloadInfo
		for _, d = range queue.Downloads {
			if d.SiaPath == downloadPath {
				break
			}
		}
		if d.Filesize == 0 {
			time.Sleep(time.Second)
			continue // file hasn't appeared in queue yet
		}
		if d.Received == d.Filesize {
			// _, err = RateLimitedCopy(chunk, tmpFile, storage.DownloadRateLimit/storage.numberOfThreads)
			fileBytes, err := ioutil.ReadFile(tmpFile.Name())
			if err != nil {
				return err
			}
			chunk.buffer.Write(fileBytes)
			os.Remove(tmpFile.Name())
			return nil
		}
		time.Sleep(time.Second)
	}
}

// UploadFile writes 'content' to the file at 'filePath'.
func (storage *SIAStorage) UploadFile(threadIndex int, filePath string, content []byte) (err error) {
	uploadPath := storage.storageDir + "/" + filePath

	h := sha256.New()
	h.Write(content)
	tmpFileName := zbase32.EncodeToString(h.Sum(nil))
	tmpFile, err := ioutil.TempFile(os.TempDir(), tmpFileName)
	tmpFile.Write(content)

	err = storage.client.Post(fmt.Sprintf("/renter/upload/%v", uploadPath), fmt.Sprintf("source=%v", tmpFile.Name()), nil)
	if err != nil {
		return err
	}

	var rf api.RenterFiles
	for {
		err = storage.client.Get("/renter/files", &rf)
		if err != nil {
			return err
		}
		for _, fi := range rf.Files {
			if fi.SiaPath == uploadPath {
				if !fi.Available {
					time.Sleep(time.Second)
				} else {
					os.Remove(tmpFile.Name())
					return nil
				}
				break
			}
		}
	}
}

// If a local snapshot cache is needed for the storage to avoid downloading/uploading chunks too often when
// managing snapshots.
func (storage *SIAStorage) IsCacheNeeded() bool { return true }

// If the 'MoveFile' method is implemented.
func (storage *SIAStorage) IsMoveFileImplemented() bool { return true }

// If the storage can guarantee strong consistency.
func (storage *SIAStorage) IsStrongConsistent() bool { return false }

// If the storage supports fast listing of files names.
func (storage *SIAStorage) IsFastListing() bool { return true }

// Enable the test mode.
func (storage *SIAStorage) EnableTestMode() {}
