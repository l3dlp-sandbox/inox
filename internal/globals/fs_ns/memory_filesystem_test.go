package fs_ns

import (
	"crypto/sha256"
	"io"
	"testing"

	billy "github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/core"
	"gopkg.in/check.v1"

	"github.com/stretchr/testify/assert"
)

var _ = check.Suite(&MemoryFsTestSuite{})

type MemoryFsTestSuite struct {
	BasicTestSuite
	DirTestSuite
}

func (s *MemoryFsTestSuite) SetUpTest(c *check.C) {
	s.BasicTestSuite = BasicTestSuite{
		FS: NewMemFilesystem(100_000_00),
	}
	s.DirTestSuite = DirTestSuite{
		FS: NewMemFilesystem(100_000_00),
	}
}

func TestMemoryFilesystem(t *testing.T) {
	check.Run(&MemoryFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})
}

func TestMemoryFilesystemCapabilities(t *testing.T) {
	cases := []struct {
		name     string
		caps     billy.Capability
		expected bool
	}{
		{
			name:     "not lock capable",
			caps:     billy.LockCapability,
			expected: false,
		},
		{
			name:     "read capable",
			caps:     billy.ReadCapability,
			expected: true,
		},
		{
			name:     "read+write capable",
			caps:     billy.ReadCapability | billy.WriteCapability,
			expected: true,
		},
		{
			name:     "read+write+truncate capable",
			caps:     billy.ReadCapability | billy.WriteCapability | billy.ReadAndWriteCapability | billy.TruncateCapability,
			expected: true,
		},
		{
			name:     "not read+write+truncate+lock capable",
			caps:     billy.ReadCapability | billy.WriteCapability | billy.ReadAndWriteCapability | billy.TruncateCapability | billy.LockCapability,
			expected: false,
		},
		{
			name:     "not truncate+lock capable",
			caps:     billy.TruncateCapability | billy.LockCapability,
			expected: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fs := NewMemFilesystem(10_000_000)
			assert.Equal(t, testCase.expected, billy.CapabilityCheck(fs, testCase.caps))
		})
	}
}

func TestMemoryFilesystemSnapshot(t *testing.T) {
	const MAX_STORAGE_SIZE = 10_000
	getContentNoCache := func(ChecksumSHA256 [32]byte) AddressableContent {
		return nil
	}

	t.Run("empty filesystem", func(t *testing.T) {
		fs := NewMemFilesystem(MAX_STORAGE_SIZE)
		snapshot := fs.TakeFilesystemSnapshot(getContentNoCache)

		assert.Len(t, snapshot.Metadata, 0)
		assert.Len(t, snapshot.FileContents, 0)
	})

	t.Run("file at root level", func(t *testing.T) {
		fs := NewMemFilesystem(MAX_STORAGE_SIZE)

		f, err := fs.Create("/file.txt")
		assert.NoError(t, err)
		f.Write([]byte("hello"))

		info := f.(*inMemfile).FileInfo()
		creationTime := info.CreationTime_
		modifTime := info.ModTime_
		mode := info.Mode_
		f.Close()

		snapshot := fs.TakeFilesystemSnapshot(getContentNoCache)

		if !assert.Len(t, snapshot.Metadata, 1) {
			return
		}
		if !assert.Contains(t, snapshot.Metadata, "/file.txt") {
			return
		}

		checkSum := sha256.Sum256([]byte("hello"))

		metadata := snapshot.Metadata["/file.txt"]
		assert.Equal(t, &FileMetadata{
			AbsolutePath:     "/file.txt",
			Size:             5,
			CreationTime:     creationTime,
			ModificationTime: modifTime,
			Mode:             mode,
			ChecksumSHA256:   checkSum,
		}, metadata)

		assert.Len(t, snapshot.FileContents, 1)
		if !assert.Contains(t, snapshot.FileContents, "/file.txt") {
			return
		}

		content := snapshot.FileContents["/file.txt"]
		assert.Equal(t, checkSum, content.ChecksumSHA256())
		actualContentBytes, err := io.ReadAll(content.Reader())
		assert.NoError(t, err)
		assert.Equal(t, []byte("hello"), actualContentBytes)
	})

	t.Run("two files at root level", func(t *testing.T) {
		fs := NewMemFilesystem(MAX_STORAGE_SIZE)

		//create both files
		f1, err := fs.Create("/file1.txt")
		assert.NoError(t, err)
		f1.Write([]byte("hello1"))

		info1 := f1.(*inMemfile).FileInfo()
		creationTime1 := info1.CreationTime_
		modifTime1 := info1.ModTime_
		mode1 := info1.Mode_
		f1.Close()

		f2, err := fs.Create("/file2.txt")
		assert.NoError(t, err)
		f2.Write([]byte("hello2"))
		info2 := f2.(*inMemfile).FileInfo()
		creationTime2 := info2.CreationTime_
		modifTime2 := info2.ModTime_
		mode2 := info2.Mode_
		f2.Close()

		snapshot := fs.TakeFilesystemSnapshot(getContentNoCache)

		if !assert.Len(t, snapshot.Metadata, 2) {
			return
		}
		assert.Len(t, snapshot.FileContents, 2)

		//check file 1
		if !assert.Contains(t, snapshot.Metadata, "/file1.txt") {
			return
		}

		checkSum1 := sha256.Sum256([]byte("hello1"))

		metadata1 := snapshot.Metadata["/file1.txt"]
		assert.Equal(t, &FileMetadata{
			AbsolutePath:     "/file1.txt",
			Size:             6,
			CreationTime:     creationTime1,
			ModificationTime: modifTime1,
			Mode:             mode1,
			ChecksumSHA256:   checkSum1,
		}, metadata1)

		if !assert.Contains(t, snapshot.FileContents, "/file1.txt") {
			return
		}

		content := snapshot.FileContents["/file1.txt"]
		assert.Equal(t, checkSum1, content.ChecksumSHA256())
		actualContentBytes, err := io.ReadAll(content.Reader())
		assert.NoError(t, err)
		assert.Equal(t, []byte("hello1"), actualContentBytes)

		//check file 2

		if !assert.Contains(t, snapshot.Metadata, "/file2.txt") {
			return
		}

		checkSum2 := sha256.Sum256([]byte("hello2"))

		metadata2 := snapshot.Metadata["/file2.txt"]
		assert.Equal(t, &FileMetadata{
			AbsolutePath:     "/file2.txt",
			Size:             6,
			CreationTime:     creationTime2,
			ModificationTime: modifTime2,
			Mode:             mode2,
			ChecksumSHA256:   checkSum2,
		}, metadata2)

		if !assert.Contains(t, snapshot.FileContents, "/file2.txt") {
			return
		}

		content2 := snapshot.FileContents["/file2.txt"]
		assert.Equal(t, checkSum2, content2.ChecksumSHA256())
		actualContentBytes2, err := io.ReadAll(content2.Reader())
		assert.NoError(t, err)
		assert.Equal(t, []byte("hello2"), actualContentBytes2)
	})

	t.Run("empty dir at root level", func(t *testing.T) {
		fs := NewMemFilesystem(MAX_STORAGE_SIZE)

		err := fs.MkdirAll("/dir", DEFAULT_DIR_FMODE)
		assert.NoError(t, err)

		info, err := fs.ReadDir("/")
		assert.NoError(t, err)
		dirInfo := info[0].(core.FileInfo)

		snapshot := fs.TakeFilesystemSnapshot(getContentNoCache)

		if !assert.Len(t, snapshot.Metadata, 1) {
			return
		}
		if !assert.Contains(t, snapshot.Metadata, "/dir") {
			return
		}

		metadata := snapshot.Metadata["/dir"]
		assert.Equal(t, &FileMetadata{
			AbsolutePath:     "/dir",
			CreationTime:     dirInfo.CreationTime_,
			ModificationTime: dirInfo.ModTime_,
			Mode:             dirInfo.Mode_,
		}, metadata)

		assert.Empty(t, snapshot.FileContents)
	})
}
