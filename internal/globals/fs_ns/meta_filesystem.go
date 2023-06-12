package fs_ns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

const (
	METAFS_UNDERLYING_FILE_PROPNAME = "underlying-file"
	METAFS_FILE_MODE_PROPNAME       = "file-mode"
	METAFS_CREATION_TIME_PROPNAME   = "creation-time"
	METAFS_MODIF_TIME_PROPNAME      = "modification-time"
	METAFS_SYMLINK_TARGET_PROPNAME  = "symlink-target"
	METAFS_CHILDREN_PROPNAME        = "children"

	METAFS_UNDERLYING_UNDERLYING_FILE_PERM = 0600

	METAFS_FILES_KEY = "/files"

	METAFS_KV_FILENAME = "metadata.kv"
)

var (
	REQUIRED_METAFS_FILE_METADATA_PROPNAMES = []string{METAFS_FILE_MODE_PROPNAME, METAFS_CREATION_TIME_PROPNAME, METAFS_MODIF_TIME_PROPNAME}
)

// MetaFilesystem is a filesystem that works on top of another filesystem, it stores its metadata in a file and file contents
// in regular files.
type MetaFilesystem struct {
	underlying afs.Filesystem
	dir        string

	//all the metadata about files is stored in this Key value store.
	// the root directory '/' has no metadata.
	metadata *filekv.SingleFileKV
	ctx      *core.Context

	lock sync.RWMutex
}

func OpenMetaFilesystem(ctx *core.Context, underlying afs.Filesystem, dir string) (*MetaFilesystem, error) {
	if err := underlying.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory for meta filesystem: %w", err)
	}

	kv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path:       core.PathFrom(underlying.Join(dir, METAFS_KV_FILENAME)),
		Filesystem: underlying,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open/create single-file KV store for storing metadata of meta filesystem: %w", err)
	}

	return &MetaFilesystem{
		ctx:        ctx,
		underlying: underlying,
		dir:        dir,
		metadata:   kv,
	}, nil
}

func (fls *MetaFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fls *MetaFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}

func (fls *MetaFilesystem) Absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return "", core.ErrNotImplemented
}

func (fls *MetaFilesystem) getFileMetadata(pth core.Path, usedTx *filekv.DatabaseTx) (*metaFsFileMetadata, bool, error) {
	if !pth.IsAbsolute() {
		return nil, false, errors.New("file's path should be absolute")
	}

	key := getKvKeyFromPath(pth)

	var (
		info core.Value
		ok   core.Bool
		err  error
	)

	if usedTx == nil {
		info, ok, err = fls.metadata.Get(fls.ctx, key, fls)
	} else {
		info, ok, err = usedTx.Get(fls.ctx, key)
	}

	if err != nil {
		return nil, false, fmtFailedToGetFileMetadataError(pth, err)
	}

	if !ok {
		return nil, false, nil
	}

	record, ok := info.(*core.Record)
	if !ok {
		return nil, false, fmt.Errorf("invalid type for metadata of file %s: %T", pth, info)
	}

	for _, propName := range REQUIRED_METAFS_FILE_METADATA_PROPNAMES {
		if !record.HasProp(fls.ctx, propName) {
			return nil, false,
				fmt.Errorf("invalid record for metadata of file %s, missing .%s property: %s", pth, propName, core.Stringify(record, fls.ctx))
		}
	}

	fileMode := record.Prop(fls.ctx, METAFS_FILE_MODE_PROPNAME).(core.FileMode)
	creationTime := record.Prop(fls.ctx, METAFS_CREATION_TIME_PROPNAME).(core.Date)
	modifTime := record.Prop(fls.ctx, METAFS_MODIF_TIME_PROPNAME).(core.Date)

	var symlinkTarget *core.Path
	if record.HasProp(fls.ctx, METAFS_SYMLINK_TARGET_PROPNAME) {
		symlinkTarget = new(core.Path)
		*symlinkTarget = record.Prop(fls.ctx, METAFS_SYMLINK_TARGET_PROPNAME).(core.Path)
	}

	var underlyingFilePath *core.Path
	if record.HasProp(fls.ctx, METAFS_UNDERLYING_FILE_PROPNAME) {
		underylingFile := record.Prop(fls.ctx, METAFS_UNDERLYING_FILE_PROPNAME).(core.Str)

		underlyingFilePath = new(core.Path)
		*underlyingFilePath = core.PathFrom(fls.underlying.Join(fls.dir, string(underylingFile)))
	}

	metadata := &metaFsFileMetadata{
		path:             pth,
		concreteFile:     underlyingFilePath,
		mode:             fs.FileMode(fileMode),
		creationTime:     creationTime,
		modificationTime: modifTime,

		symlinkTarget: symlinkTarget,
	}

	return metadata, true, nil
}

func (fls *MetaFilesystem) setFileMetadata(metadata *metaFsFileMetadata, usedTx *filekv.DatabaseTx) error {
	if !metadata.path.IsAbsolute() {
		return errors.New("file's path should be absolute")
	}

	recordPropertyNames := []string{
		METAFS_FILE_MODE_PROPNAME,
		METAFS_CREATION_TIME_PROPNAME,
		METAFS_MODIF_TIME_PROPNAME,
	}

	recordPropertyValues := []core.Value{

		core.FileMode(metadata.mode),
		metadata.creationTime,
		metadata.modificationTime,
	}

	if metadata.mode.IsDir() {
		var children []core.Value

		for _, path := range metadata.children {
			children = append(children, path)
		}

		recordPropertyNames = append(recordPropertyNames, METAFS_CHILDREN_PROPNAME)
		recordPropertyValues = append(recordPropertyValues, core.NewTuple(children))
	} else { //if not a dir set name of underlying file
		recordPropertyNames = append(recordPropertyNames, METAFS_UNDERLYING_FILE_PROPNAME)
		recordPropertyValues = append(recordPropertyValues, core.Str(metadata.concreteFile.Basename()))
	}

	metadataRecord := core.NewRecordFromKeyValLists(recordPropertyNames, recordPropertyValues)

	key := getKvKeyFromPath(metadata.path)

	if usedTx == nil {
		fls.metadata.Set(fls.ctx, key, metadataRecord, fls)
	} else {
		return usedTx.Set(fls.ctx, key, metadataRecord)
	}

	return nil
}

func (fls *MetaFilesystem) deleteFileMetadata(pth core.Path, usedTx *filekv.DatabaseTx) error {
	key := getKvKeyFromPath(pth)

	if usedTx == nil {
		fls.metadata.Delete(fls.ctx, key, fls)
	} else {
		return usedTx.Delete(fls.ctx, key)
	}

	return nil
}

func (fls *MetaFilesystem) Create(filename string) (billy.File, error) {
	return fls.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (fls *MetaFilesystem) Open(filename string) (billy.File, error) {
	return fls.OpenFile(filename, os.O_RDONLY, 0)
}

func (fls *MetaFilesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	fls.lock.Lock()
	defer fls.lock.Unlock()

	originalPath := filename
	filename = normalizeAsAbsolute(filename)

	pth := core.PathFrom(filename)
	metadata, exists, err := fls.getFileMetadata(pth, nil)
	if err != nil {
		return nil, err
	}

	if !exists {
		if !isCreate(flag) {
			return nil, os.ErrNotExist
		}

		dir := filepath.Dir(originalPath)
		if dir != "/" {
			//make sure parent exists
			err := fls.MkdirAllNoLock(dir, 0700)
			if err != nil {
				return nil, fmt.Errorf("failed to create %s", dir)
			}
		}

		//get & update metadata of parent directory
		dirPath := filepath.Dir(string(pth))
		if dirPath != "/" {
			dirMetadata, found, err := fls.getFileMetadata(core.PathFrom(dirPath), nil)
			if err != nil {
				return nil, err
			}

			if !found {
				return nil, fmt.Errorf("failed to create %s: parent directory %s does not exist", pth, dirPath)
			}
			dirMetadata.children = append(dirMetadata.children, pth)
			if err := fls.setFileMetadata(dirMetadata, nil); err != nil {
				return nil, err
			}
		}

		//create & store metadata for new file

		underlyingFilePath := core.Path(fls.underlying.Join(fls.dir, ulid.Make().String()))
		creationTime := core.Date(time.Now())

		mode := fs.FileMode(perm)

		newFileMetadata := &metaFsFileMetadata{
			path:             pth,
			concreteFile:     &underlyingFilePath,
			mode:             mode,
			creationTime:     creationTime,
			modificationTime: creationTime,
		}

		if err := fls.setFileMetadata(newFileMetadata, nil); err != nil {
			return nil, err
		}

		metadata = newFileMetadata
	} else {
		if isSymlink(metadata.mode) {
			//
			return nil, errors.New("symlinks not supported")
		}

		if isExclusive(flag) {
			return nil, os.ErrExist
		}
	}

	underlyingFile, err := fls.underlying.OpenFile(metadata.concreteFile.UnderlyingString(), flag, METAFS_UNDERLYING_UNDERLYING_FILE_PERM)

	if err != nil {
		//TODO: give more info about the error without leaking information about the underlying filesystem.
		return nil, fmt.Errorf("failed to open %s", pth)
	}

	if metadata.mode.IsDir() {
		return nil, fmt.Errorf("cannot open directory: %s", filename)
	}

	file := &metaFsFile{
		path:         pth,
		fs:           fls,
		originalPath: originalPath,
		metadata:     metadata,
		underlying:   underlyingFile,
	}

	return file, nil
}

func (fls *MetaFilesystem) Stat(filename string) (os.FileInfo, error) {
	fls.lock.RLock()
	defer fls.lock.RUnlock()

	return fls.statNoLock(filename)
}

func (fls *MetaFilesystem) statNoLock(filename string) (os.FileInfo, error) {
	filename = normalizeAsAbsolute(filename)

	metadata, exists, err := fls.getFileMetadata(core.PathFrom(filename), nil)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	var size core.ByteCount

	if metadata.concreteFile != nil {
		underlyingFilePath := *metadata.concreteFile
		stat, err := fls.underlying.Stat(string(underlyingFilePath))
		if err != nil {
			return nil, fmt.Errorf("failed to get stat of %s", filename)
		}
		size = core.ByteCount(stat.Size())
	}

	return core.FileInfo{
		BaseName_:       string(metadata.path.Basename()),
		AbsPath_:        metadata.path,
		Mode_:           core.FileMode(metadata.mode),
		CreationTime_:   metadata.creationTime,
		ModTime_:        metadata.modificationTime,
		HasCreationTime: true,
		Size_:           size,
	}, nil
}

func (fls *MetaFilesystem) Lstat(filename string) (os.FileInfo, error) {
	fls.lock.RLock()
	defer fls.lock.RUnlock()

	metadata, exists, err := fls.getFileMetadata(core.PathFrom(filename), nil)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	if isSymlink(metadata.mode) {
		return nil, errors.New("symlinks not supported")
	}

	return fls.statNoLock(filename)
}

func (fls *MetaFilesystem) ReadDir(path string) ([]os.FileInfo, error) {
	fls.lock.RLock()
	defer fls.lock.RUnlock()

	path = normalizeAsAbsolute(path)

	metadata, exists, err := fls.getFileMetadata(core.PathFrom(path), nil)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	if !metadata.mode.IsDir() {
		return nil, errors.New("not a dir")
	}

	var entries []os.FileInfo
	for _, child := range metadata.children {
		stat, err := fls.statNoLock(child.UnderlyingString())
		if err != nil {
			return nil, err
		}
		entries = append(entries, stat)
	}

	sort.Sort(SortableFileInfo(entries))

	return entries, nil
}

func (fls *MetaFilesystem) MkdirAll(path string, perm os.FileMode) error {
	fls.lock.Lock()
	defer fls.lock.Unlock()

	return fls.MkdirAllNoLock(path, perm)
}

func (fls *MetaFilesystem) MkdirAllNoLock(path string, perm os.FileMode) error {
	if path == "/" {
		return nil
	}

	path = normalizeAsAbsolute(path)
	perm |= fs.ModeDir

	_, exists, err := fls.getFileMetadata(core.PathFrom(path), nil)

	if err != nil {
		return err
	}

	pth := core.DirPathFrom(path)

	if !exists { //create the directory

		//make sure the parent exists
		dir := filepath.Dir(path)
		if dir != "/" && dir != "." {
			err := fls.MkdirAllNoLock(dir, perm)
			if err != nil {
				return err
			}
		}

		creationTime := core.Date(time.Now())

		newFileMetadata := &metaFsFileMetadata{
			path:             pth,
			mode:             perm,
			creationTime:     creationTime,
			modificationTime: creationTime,
		}

		if err := fls.setFileMetadata(newFileMetadata, nil); err != nil {
			return err
		}
	}

	//TODO: support creating intermediary directories

	return nil
}

func (fls *MetaFilesystem) TempFile(dir, prefix string) (billy.File, error) {
	return nil, core.ErrNotImplementedYet
}

func (fls *MetaFilesystem) Rename(from, to string) error {
	fls.lock.Lock()
	defer fls.lock.Unlock()

	from = normalizeAsAbsolute(from)
	to = normalizeAsAbsolute(to)

	_, exists, err := fls.getFileMetadata(core.PathFrom(from), nil)

	if err != nil {
		return err
	}

	if !exists {
		return os.ErrNotExist
	}

	fromPath := core.PathFrom(from)
	toPath := core.PathFrom(to)

	from = fromPath.UnderlyingString()
	to = toPath.UnderlyingString()

	move := [][2]core.Path{{fromPath, toPath}}

	filesPrefix := METAFS_FILES_KEY + "/"

	err = fls.metadata.ForEach(fls.ctx, func(key core.Path, getVal func() core.Value) error {
		path := strings.TrimPrefix(string(key), filesPrefix)

		if path == string(key) { //prefix not present
			return nil
		}

		if path == from || !filepath.HasPrefix(path, from) {
			return nil
		}

		rel, _ := filepath.Rel(from, path)
		pathTo := filepath.Join(to, rel)

		move = append(move, [2]core.Path{core.PathFrom(path), core.PathFrom(pathTo)})
		return nil
	}, fls)

	if err != nil {
		return err
	}

	noCheckFuel := 10

	err = fls.metadata.UpdateNoCtx(func(dbTx *filekv.DatabaseTx) error {
		fromDir := filepath.Dir(from)
		if fromDir != "/" && fromDir != "." {
			// get metadata of previous parent directory
			fromDirPath := core.DirPathFrom(fromDir)

			fromDirMetadata, found, err := fls.getFileMetadata(fromDirPath, dbTx)
			if err != nil {
				return err
			}

			if !found {
				panic(core.ErrUnreachable)
			}

			// update it
			indexFound := false
			for index, child := range fromDirMetadata.children {
				if child == fromPath {
					indexFound = true
					fromDirMetadata.children = utils.RemoveIndexOfSlice(fromDirMetadata.children, index)
					break
				}
			}

			if !indexFound {
				return fmt.Errorf("failed to remove %s from children of %s", fromPath.Basename(), fromDirPath)
			}

			if err := fls.setFileMetadata(fromDirMetadata, dbTx); err != nil {
				return err
			}
		}

		//update metadata of moved files & directories

		for _, ops := range move {

			if noCheckFuel <= 0 { //check context
				select {
				case <-fls.ctx.Done():
					return fls.ctx.Err()
				default:
				}
				noCheckFuel = 10
			} else {
				noCheckFuel--
			}

			from := ops[0]
			to := ops[1]

			//get current metadata
			metadata, exists, err := fls.getFileMetadata(from, dbTx)
			if err != nil {
				return err
			}
			if !exists {
				panic(core.ErrUnreachable)
			}

			//update the metadata.
			//note that we do not need to update the underlying file since it
			//only contains the content.
			metadata.path = to

			err = fls.setFileMetadata(metadata, dbTx)
			if err != nil {
				return err
			}

			//delete previous metadata
			fls.deleteFileMetadata(from, dbTx)
		}
		return nil
	})

	return err
}

func (fls *MetaFilesystem) Remove(filename string) error {
	fls.lock.Lock()
	defer fls.lock.Unlock()

	filename = normalizeAsAbsolute(filename)

	pth := core.PathFrom(filename)
	metadata, exists, err := fls.getFileMetadata(pth, nil)
	if err != nil {
		return err
	}
	if !exists {
		return os.ErrNotExist
	}

	noCheckFuel := 10

	err = fls.metadata.UpdateNoCtx(func(dbTx *filekv.DatabaseTx) error {
		dir := filepath.Dir(filename)

		//remove entry from parent
		if dir != "/" && dir != "." {
			parentMetadata, exists, err := fls.getFileMetadata(pth, dbTx)
			if err != nil {
				return err
			}
			if !exists {
				panic(core.ErrUnreachable)
			}

			found := false
			for index, child := range parentMetadata.children {
				if child == pth {
					parentMetadata.children = utils.RemoveIndexOfSlice(parentMetadata.children, index)
					break
				}
			}
			if !found {
				panic(core.ErrUnreachable)
			}

			if err := fls.setFileMetadata(parentMetadata, dbTx); err != nil {
				return err
			}
		}

		if err := fls.deleteFileMetadata(metadata.path, dbTx); err != nil {
			return err
		}

		if !metadata.mode.IsDir() {
			return nil
		}

		//remove descendants recursively
		queue := utils.CopySlice(metadata.children)

		for len(queue) > 0 {
			if noCheckFuel <= 0 { //check context
				select {
				case <-fls.ctx.Done():
					return fls.ctx.Err()
				default:
				}
				noCheckFuel = 10
			} else {
				noCheckFuel--
			}

			current := queue[len(queue)-1]
			queue = queue[:len(queue)-1]

			currentMetadata, exists, err := fls.getFileMetadata(current, dbTx)

			if err != nil {
				return err
			}

			if !exists {
				//the metadata should exist, continue anyway
				continue
			}

			//delete current descendant & add its own descendants to the queue
			if currentMetadata.mode.IsDir() {
				queue = append(queue, currentMetadata.children...)
			}

			if err := fls.deleteFileMetadata(metadata.path, dbTx); err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (fls *MetaFilesystem) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fls *MetaFilesystem) Symlink(target, link string) error {
	return core.ErrNotImplementedYet
}

func (fls *MetaFilesystem) Readlink(link string) (string, error) {
	return "", core.ErrNotImplementedYet
}

type metaFsFileMetadata struct {
	path             core.Path
	concreteFile     *core.Path //nil if dir
	mode             fs.FileMode
	creationTime     core.Date
	modificationTime core.Date

	//the targets of symlinks are directly stored in the metadata,
	//there is no underlying file.
	symlinkTarget *core.Path

	//children files if directory
	children []core.Path
}

func getKvKeyFromPath(pth core.Path) core.Path {
	key := METAFS_FILES_KEY + pth
	//remove trailing slash
	if key[len(key)-1] == '/' {
		key = key[:len(key)-1]
	}

	return key
}

func fmtFailedToGetFileMetadataError(pth core.Path, err error) error {
	return fmt.Errorf("failed to get metadata for file %s: %w", pth, err)
}
