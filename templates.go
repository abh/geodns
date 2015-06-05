package main

import (
	"bytes"
	"compress/gzip"
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

type _escFile struct {
	compressed string
	size       int64
	local      string
	isDir      bool

	data []byte
	once sync.Once
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
		gr, err = gzip.NewReader(bytes.NewBufferString(f.compressed))
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
	return time.Time{}
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
		local: "templates/status.html",
		size:  3181,
		compressed: "" +
			"\x1f\x8b\b\x00\x00\tn\x88\x00\xff\xb4WMS\xeb6\x14]'\xbfB\xe3)\xbb\"\x13Jf\x1a\xeax\x03\f]4\x14\x9a\xb6\x8b\xb7S\xac\x9b\xd8\xf3\x1c)OR\b\xbcL\xfe\xfb\xbb\x92?b;&\x04\xbfa\x83\xe5#\x9d+\x9d{O\xaeL\x10\x9be\x1a\x0610\x1e\x06&1)\x84\xf7 o\x1f\xa6d\xbb%\xf4\u007fP:\x91\x82" +
			"\xecv\x81\x9fM\xf6\x834\x11_I\xac`>\xf6|_\x80\xe1\x82љ\x94F\x1b\xc5V\x11\x174\x92K\xdfl\x12c@\x9d\x97\x13\xfe%\xfd\x8d\x0e\xfcHk\xbf\xc4\xceq\xe5,\x11\xc0\xe92A\x9a\xd6\x1eQ\x90\x8e=m^S\xd01\x80\xf1N\xdd\xcf\x01\x1bf\xa2\xb8\xd8\b\xd4:\x05&\xf6\xbb\x1d\xddĽ\x85\xfd\x9e\xe1\xf4\xbb\x14" +
			" \xd8\x12~űM\v(\xb2%s)\xcc\xf9\x06\x92El\xae\xc9L\xa6\xfc\x0f\xb2\xeb\a~N\vf\x92\xbf\x86\xfd~\xc0\x93g\x12\xa5L\xeb\xb1\x17!\x83\xa18\xe5ىx\x10ާr\xc6\xd2\xc0\xc7a}\xa5\x92\x1b\xbb\xa6W\xc5\xf4\x8a\x89\xdf\x1d\xda\v\f\x9b\xa5PLd/\xee/&W\xe1\xe9\x80篸#\a\xa1\x81#" +
			"\xafgy\xca=q\xc0\vv\"\xe6\x92d\xa2\xbc\xfc@D\x1bf4\x96\x97\x97\xab\xc3\x01\xc1\\ա\xe1!4h\xc1&\x98\xf2\x12\xc1\x81r\x12jg\t\x9f֠\x12hl\xb9ݮT\"̜xg\xf4r\xee\x11\x9a\x9d\x8e\xe6\x8b\xe9?\xcc\xc0\xc0\xd9\xf0\x83\xa4a\x17Ҡ\x13ˊ\xaf\xf02\xf9\xeei\xeb\xf3\xb9\xc5\xfc3\xd1" +
			"F.\x14[6\xf2:9Z!\x87\x8c.\xce\x1a\xc0\xe8\x00\xa0Mh\xc2^\xea\xc0\xd4\xf0[x~\xaf\xf6\xb9羽c\x01\xee\x11R\xa6\xb8\x94FQ̩u\xa9\x90\xeaU9\x91\xf5\x18\x99\xd1\x05\xe9\xc6\x1bu\xe5\x1d%\xbe\x95\x13\xf6\xf2\xf1ݲb\x91\x03\xb3\xd6\xdd\x1a\xf8ؑ\xda:\xd3\xd5Ou\xa6\xc2\x10\x15+\x17.\xfeo" +
			"e\x92%\xec\x0f\xe5\xc4\xd0\f\xa5\xb7\xecujP\xd8b\u007f\xec\xf2'\xf6v\xc8ǔ\x99\xb9T\xcbf\xd0\x02o\tv\x98\x81\xf2\x89\xcd\xfb\v\xde\x0f:k\xe3\x9d\x12\xe0\xce\x1a\x988\xc4}\xe2l\xe4\xfa\xed\xda\xc0\x1e\x19戮,j\xc1\xac\xb7\xc9\xd3\xe34G\xb2\x1a\xe2\xc5\xfd\x8b\x91\xab\xbf1kxu_\x8f\t\xfd\xb7x\xdb\xed" +
			"\xec\xb4bb\x01\xe8%\xa7\x84X\xcc&\xb0\xdaV\xf2:a2ej\v>\x1e\x16\xb3\xc5\xed\xe8\xd9\x1c>\xe0\xa0\xcc_ᠠ(\xc9A\xa7o\xf5\xe6\x04\xb0\xa6\x91~\xabџ\xcc\x19v\xe0\f\xba\x90j\xfd\xa4E\xf4\x9d\xfdx\xfa\x88\xf2;.t\x17\xf5Mީb\x0e\xf6\xebJl̈́u_$\xd7\xc2hg\xbd\x82\xfb\x17\x9b" +
			"A:\xb5\x9f\x1a֍7ق\x8aM\x9d\v\x91\x9b\xccK:B\xbdVc֝\x19\xbaк\xf9+.l^\x8bָG\xf3\x10W\xd6\xcb.J\xae&o\x11\xee\x94-\x17\xfav\v\x82\xdbx\xc5 W\x9d&Д}\xe3\xb0Su\xe7\x01N\x15\x9e\x05?\xa2\xbc\x1aﳤ\xf7\xcbѾo\xee\xdbf\xf6a\x8c\xdd\xd2\xfe\x97\xd1" +
			"\xff\x11\x00\x00\xff\xff\xb0\x9dR\xc9m\f\x00\x00",
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
