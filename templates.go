package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

type _escLocalFS struct{}

var _escLocal _escLocalFS

type _escStaticFS struct{}

var _escStatic _escStaticFS

type _escDirectory struct {
	fs   http.FileSystem
	name string
}

type _escFile struct {
	compressed string
	size       int64
	modtime    int64
	local      string
	isDir      bool

	once sync.Once
	data []byte
	name string
}

func (_escLocalFS) Open(name string) (http.File, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	return os.Open(f.local)
}

func (_escStaticFS) prepare(name string) (*_escFile, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	f.once.Do(func() {
		f.name = path.Base(name)
		if f.size == 0 {
			return
		}
		var gr *gzip.Reader
		b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.compressed))
		gr, err = gzip.NewReader(b64)
		if err != nil {
			return
		}
		f.data, err = ioutil.ReadAll(gr)
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs _escStaticFS) Open(name string) (http.File, error) {
	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File()
}

func (dir _escDirectory) Open(name string) (http.File, error) {
	return dir.fs.Open(dir.name + name)
}

func (f *_escFile) File() (http.File, error) {
	type httpFile struct {
		*bytes.Reader
		*_escFile
	}
	return &httpFile{
		Reader:   bytes.NewReader(f.data),
		_escFile: f,
	}, nil
}

func (f *_escFile) Close() error {
	return nil
}

func (f *_escFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

func (f *_escFile) Stat() (os.FileInfo, error) {
	return f, nil
}

func (f *_escFile) Name() string {
	return f.name
}

func (f *_escFile) Size() int64 {
	return f.size
}

func (f *_escFile) Mode() os.FileMode {
	return 0
}

func (f *_escFile) ModTime() time.Time {
	return time.Unix(f.modtime, 0)
}

func (f *_escFile) IsDir() bool {
	return f.isDir
}

func (f *_escFile) Sys() interface{} {
	return f
}

// FS returns a http.Filesystem for the embedded assets. If useLocal is true,
// the filesystem's contents are instead used.
func FS(useLocal bool) http.FileSystem {
	if useLocal {
		return _escLocal
	}
	return _escStatic
}

// Dir returns a http.Filesystem for the embedded assets on a given prefix dir.
// If useLocal is true, the filesystem's contents are instead used.
func Dir(useLocal bool, name string) http.FileSystem {
	if useLocal {
		return _escDirectory{fs: _escLocal, name: name}
	}
	return _escDirectory{fs: _escStatic, name: name}
}

// FSByte returns the named file from the embedded assets. If useLocal is
// true, the filesystem's contents are instead used.
func FSByte(useLocal bool, name string) ([]byte, error) {
	if useLocal {
		f, err := _escLocal.Open(name)
		if err != nil {
			return nil, err
		}
		return ioutil.ReadAll(f)
	}
	f, err := _escStatic.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.data, nil
}

// FSMustByte is the same as FSByte, but panics if name is not present.
func FSMustByte(useLocal bool, name string) []byte {
	b, err := FSByte(useLocal, name)
	if err != nil {
		panic(err)
	}
	return b
}

// FSString is the string version of FSByte.
func FSString(useLocal bool, name string) (string, error) {
	b, err := FSByte(useLocal, name)
	return string(b), err
}

// FSMustString is the string version of FSMustByte.
func FSMustString(useLocal bool, name string) string {
	return string(FSMustByte(useLocal, name))
}

var _escData = map[string]*_escFile{

	"/templates/status.html": {
		local:   "templates/status.html",
		size:    3181,
		modtime: 1442049075,
		compressed: `
H4sIAAAJbogA/7RXTVPrNhRdJ79C4ym7IhNKZhrqeAMMXTQUmraLt1Osm9jzHClPUgi8TP77u5I/Yjsm
BL9hg+UjnSude0+uTBCbZRoGMTAeBiYxKYT3IG8fpmS7JfR/UDqRgux2gZ9N9oM0EV9JrGA+9nxfgOGC
0ZmURhvFVhEXNJJL32wSY0CdlxP+Jf2NDvxIa7/EznHlLBHA6TJBmtYeUZCOPW1eU9AxgPFO3c8BG2ai
uNgI1DoFJva7Hd3EvYX9nuH0uxQg2BJ+xbFNCyiyJXMpzPkGkkVsrslMpvwPsusHfk4LZpK/hv1+wJNn
EqVM67EXIYOhOOXZiXgQ3qdyxtLAx2F9pZIbu6ZXxfSKid8d2gsMm6VQTGQv7i8mV+HpgOevuCMHoYEj
r2d5yj1xwAt2IuaSZKK8/EBEG2Y0lpeXq8MBwVzVoeEhNGjBJpjyEsGBchJqZwmf1qASaGy53a5UIsyc
eGf0cu4Rmp2O5ovpP8zAwNnwg6RhF9KgE8uKr/Ay+e5p6/O5xfwz0UYuFFs28jo5WiGHjC7OGsDoAKBN
aMJe6sDU8Ft4fq/2uee+vWMB7hFSpriURlHMqXWpkOpVOZH1GJnRBenGG3XlHSW+lRP28vHdsmKRA7PW
3Rr42JHaOtPVT3WmwhAVKxcu/m9lkiXsD+XE0Aylt+x1alDYYn/s8if2dsjHlJm5VMtm0AJvCXaYgfKJ
zfsL3g86a+OdEuDOGpg4xH3ibOT67drAHhnmiK4sasGst8nT4zRHshrixf2Lkau/MWt4dV+PCf23eNvt
7LRiYgHoJaeEWMwmsNpW8jphMmVqCz4eFrPF7ejZHD7goMxf4aCgKMlBp2/15gSwppF+q9GfzBl24Ay6
kGr9pEX0nf14+ojyOy50F/VN3qliDvbrSmzNhHVfJNfCaGe9gvsXm0E6tZ8a1o032YKKTZ0LkZvMSzpC
vVZj1p0ZutC6+SsubF6L1rhH8xBX1ssuSq4mbxHulC0X+nYLgtt4xSBXnSbQlH3jsFN15wFOFZ4FP6K8
Gu+zpPfL0b5v7ttm9mGM3dL+l9H/EQAA//+wnVLJbQwAAA==
`,
	},

	"/": {
		isDir: true,
		local: "/",
	},

	"/templates": {
		isDir: true,
		local: "/templates",
	},
}
