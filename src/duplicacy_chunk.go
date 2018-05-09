// Copyright (c) Acrosync LLC. All rights reserved.
// Free for personal use and commercial trial
// Commercial use requires per-user licenses available from https://duplicacy.com

package duplicacy

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"hash"
	"os"
	"runtime"

	"github.com/aead/skein"
	"github.com/aead/skein/threefish"
	"github.com/lhecker/argon2"
	"github.com/pbtrung/zstd"
	"github.com/tv42/zbase32"
	"golang.org/x/crypto/pbkdf2"
)

// A chunk needs to acquire a new buffer and return the old one for every encrypt/decrypt operation, therefore
// we maintain a pool of previously used buffers.
var chunkBufferPool chan *bytes.Buffer = make(chan *bytes.Buffer, runtime.NumCPU()*16)

func AllocateChunkBuffer() (buffer *bytes.Buffer) {
	select {
	case buffer = <-chunkBufferPool:
	default:
		buffer = new(bytes.Buffer)
	}
	return buffer
}

func ReleaseChunkBuffer(buffer *bytes.Buffer) {
	select {
	case chunkBufferPool <- buffer:
	default:
		LOG_INFO("CHUNK_BUFFER", "Discarding a free chunk buffer due to a full pool")
	}
}

// Chunk is the object being passed between the chunk maker, the chunk uploader, and chunk downloader.  It can be
// read and written like a bytes.Buffer, and provides convenient functions to calculate the hash and id of the chunk.
type Chunk struct {
	buffer *bytes.Buffer // Where the actual data is stored.  It may be nil for hash-only chunks, where chunks
	// are only used to compute the hashes

	size int // The size of data stored.  This field is needed if buffer is nil

	hasher hash.Hash // Keeps track of the hash of data stored in the buffer.  It may be nil, since sometimes
	// it isn't necessary to compute the hash, for instance, when the encrypted data is being
	// read into the primary buffer

	hash []byte // The hash of the chunk data.  It is always in the binary format
	id   string // The id of the chunk data (used as the file name for saving the chunk); always in hex format

	config *Config // Every chunk is associated with a Config object.  Which hashing algorithm to use is determined
	// by the config
}

// Magic word to identify a duplicacy format encrypted file, plus a version number.
var ENCRYPTION_HEADER = "duplicacy\000"

// CreateChunk creates a new chunk.
func CreateChunk(config *Config, bufferNeeded bool) *Chunk {

	var buffer *bytes.Buffer

	if bufferNeeded {
		buffer = AllocateChunkBuffer()
		buffer.Reset()
		if buffer.Cap() < config.MaximumChunkSize {
			buffer.Grow(config.MaximumChunkSize - buffer.Cap())
		}
	}

	return &Chunk{
		buffer: buffer,
		config: config,
	}
}

// GetLength returns the length of available data
func (chunk *Chunk) GetLength() int {
	if chunk.buffer != nil {
		return len(chunk.buffer.Bytes())
	} else {
		return chunk.size
	}
}

// GetBytes returns data available in this chunk
func (chunk *Chunk) GetBytes() []byte {
	return chunk.buffer.Bytes()
}

// Reset makes the chunk reusable by clearing the existing data in the buffers.  'hashNeeded' indicates whether the
// hash of the new data to be read is needed.  If the data to be read in is encrypted, there is no need to
// calculate the hash so hashNeeded should be 'false'.
func (chunk *Chunk) Reset(hashNeeded bool) {
	if chunk.buffer != nil {
		chunk.buffer.Reset()
	}
	if hashNeeded {
		chunk.hasher = chunk.config.NewKeyedHasher(chunk.config.HashKey)
	} else {
		chunk.hasher = nil
	}
	chunk.hash = nil
	chunk.id = ""
	chunk.size = 0
}

// Write implements the Writer interface.
func (chunk *Chunk) Write(p []byte) (int, error) {

	// buffer may be nil, when the chunk is used for computing the hash only.
	if chunk.buffer == nil {
		chunk.size += len(p)
	} else {
		chunk.buffer.Write(p)
	}

	// hasher may be nil, when the chunk is used to stored encrypted content
	if chunk.hasher != nil {
		chunk.hasher.Write(p)
	}
	return len(p), nil
}

// GetHash returns the chunk hash.
func (chunk *Chunk) GetHash() string {
	if len(chunk.hash) == 0 {
		chunk.hash = chunk.hasher.Sum(nil)
	}

	return string(chunk.hash)
}

// GetID returns the chunk id.
func (chunk *Chunk) GetID() string {
	if len(chunk.id) == 0 {
		if len(chunk.hash) == 0 {
			chunk.hash = chunk.hasher.Sum(nil)
		}

		id := pbkdf2.Key(chunk.config.IDKey, []byte(chunk.hash), 13, 64, sha512.New)
		chunk.id = zbase32.EncodeToString(id)
	}

	return chunk.id
}

func (chunk *Chunk) VerifyID() {
	hasher := chunk.config.NewKeyedHasher(chunk.config.HashKey)
	hasher.Write(chunk.buffer.Bytes())
	hash := hasher.Sum(nil)
	id := pbkdf2.Key(chunk.config.IDKey, []byte(hash), 13, 64, sha512.New)
	chunkID := zbase32.EncodeToString(id)
	if chunkID != chunk.GetID() {
		LOG_ERROR("CHUNK_ID", "The chunk id should be %s instead of %s, length: %d", chunkID, chunk.GetID(), len(chunk.buffer.Bytes()))
	}
}

func threefishCTR(encryptionKey []byte, salt []byte, src []byte, enc bool) (dst []byte, skienMac []byte, err error) {
	cfg := argon2.DefaultConfig()
	cfg.HashLength = threefish.BlockSize1024 + threefish.TweakSize + threefish.BlockSize512 + threefish.BlockSize1024
	cfg.TimeCost = 4
	cfg.MemoryCost = 1 << 15
	cfg.Parallelism = 2
	raw, err := cfg.Hash(encryptionKey, salt)
	if err != nil {
		return nil, nil, err
	}

	var t3fTweak [threefish.TweakSize]byte
	copy(t3fTweak[:], raw.Hash[threefish.BlockSize1024:threefish.BlockSize1024+threefish.TweakSize])
	t3f, err := threefish.NewCipher(&t3fTweak, raw.Hash[:threefish.BlockSize1024])
	if err != nil {
		return nil, nil, err
	}

	// Encrypt it.
	iv := raw.Hash[threefish.BlockSize1024+threefish.TweakSize+threefish.BlockSize512:]
	stream := cipher.NewCTR(t3f, iv)
	dst = make([]byte, len(src))
	stream.XORKeyStream(dst, src)

	skeinMacKey := raw.Hash[threefish.BlockSize1024+threefish.TweakSize : threefish.BlockSize1024+threefish.TweakSize+threefish.BlockSize512]
	hasher := skein.New(64, &skein.Config{Key: skeinMacKey, Personal: PERSONALIZATION})
	hasher.Write([]byte(ENCRYPTION_HEADER))
	hasher.Write(salt)
	if enc {
		hasher.Write(dst)
	} else {
		hasher.Write(src)
	}
	skienMac = hasher.Sum(nil)

	return dst, skienMac, nil
}

// Encrypt encrypts the plain data stored in the chunk buffer.  If derivationKey is not nil, the actual
// encryption key will be HMAC-SHA256(encryptionKey, derivationKey).
func (chunk *Chunk) Encrypt(encryptionKey []byte, derivationKey string) (err error) {

	var salt []byte
	var key []byte

	encryptedBuffer := AllocateChunkBuffer()
	encryptedBuffer.Reset()
	defer func() {
		ReleaseChunkBuffer(encryptedBuffer)
	}()

	if len(encryptionKey) > 0 {

		key = encryptionKey

		if len(derivationKey) > 0 {
			key = pbkdf2.Key(encryptionKey, []byte(derivationKey), 100, 128, sha512.New)
		}

		// Start with the magic number and the version number.
		encryptedBuffer.Write([]byte(ENCRYPTION_HEADER))

		// Followed by the nonce
		salt = make([]byte, 32)
		_, err := rand.Read(salt)
		if err != nil {
			return err
		}
		encryptedBuffer.Write(salt)
	}

	compressed, err := zstd.CompressLevel(nil, chunk.buffer.Bytes(), chunk.config.CompressionLevel)
	if err != nil {
		return err
	}

	if len(encryptionKey) == 0 {
		encryptedBuffer.Write(compressed)
		chunk.buffer, encryptedBuffer = encryptedBuffer, chunk.buffer
		return nil
	}

	encrypted, skeinMac, err := threefishCTR(key, salt, compressed, true)
	if err != nil {
		return err
	}
	encryptedBuffer.Write(skeinMac)
	encryptedBuffer.Write(encrypted)
	chunk.buffer, encryptedBuffer = encryptedBuffer, chunk.buffer

	return nil
}

// This is to ensure compability with Vertical Backup, which still uses HMAC-SHA256 (instead of HMAC-BLAKE2) to
// derive the key used to encrypt/decrypt files and chunks.

var DecryptWithHMACSHA256 = false

func init() {
	if value, found := os.LookupEnv("DUPLICACY_DECRYPT_WITH_HMACSHA256"); found && value != "0" {
		DecryptWithHMACSHA256 = true
	}
}

// Decrypt decrypts the encrypted data stored in the chunk buffer.  If derivationKey is not nil, the actual
// encryption key will be HMAC-SHA256(encryptionKey, derivationKey).
func (chunk *Chunk) Decrypt(encryptionKey []byte, derivationKey string) (err error) {

	var offset int

	encryptedBuffer := AllocateChunkBuffer()
	encryptedBuffer.Reset()
	defer func() {
		ReleaseChunkBuffer(encryptedBuffer)
	}()

	chunk.buffer, encryptedBuffer = encryptedBuffer, chunk.buffer

	if len(encryptionKey) > 0 {

		key := encryptionKey

		if len(derivationKey) > 0 {
			key = pbkdf2.Key(encryptionKey, []byte(derivationKey), 100, 128, sha512.New)
		}

		headerLength := len(ENCRYPTION_HEADER)
		offset = headerLength + 32 + threefish.BlockSize512

		if len(encryptedBuffer.Bytes()) < offset {
			return fmt.Errorf("No enough encrypted data (%d bytes) provided", len(encryptedBuffer.Bytes()))
		}

		if string(encryptedBuffer.Bytes()[:headerLength-1]) != ENCRYPTION_HEADER[:headerLength-1] {
			return fmt.Errorf("The storage doesn't seem to be encrypted")
		}

		if encryptedBuffer.Bytes()[headerLength-1] != 0 {
			return fmt.Errorf("Unsupported encryption version %d", encryptedBuffer.Bytes()[headerLength-1])
		}

		salt := encryptedBuffer.Bytes()[headerLength : headerLength+32]
		chunkSkeinMac := encryptedBuffer.Bytes()[headerLength+32 : offset]

		decrypted, skeinMac, err := threefishCTR(key, salt, encryptedBuffer.Bytes()[offset:], false)
		if err != nil {
			return err
		}
		if bytes.Compare(chunkSkeinMac, skeinMac) != 0 {
			return fmt.Errorf("Unable to verify MAC")
		}

		chunk.buffer.Reset()
		decompressed, err := zstd.Decompress(nil, decrypted)
		if err != nil {
			return err
		}
		chunk.buffer.Write(decompressed)
		chunk.hasher = chunk.config.NewKeyedHasher(chunk.config.HashKey)
		chunk.hasher.Write(decompressed)
		chunk.hash = nil
		return nil
	}

	chunk.buffer.Reset()
	decompressed, err := zstd.Decompress(nil, encryptedBuffer.Bytes())
	if err != nil {
		return err
	}
	chunk.buffer.Write(decompressed)
	chunk.hasher = chunk.config.NewKeyedHasher(chunk.config.HashKey)
	chunk.hasher.Write(decompressed)
	chunk.hash = nil
	return nil
}
