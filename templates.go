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
		b, err := ioutil.ReadAll(f)
		f.Close()
		return b, err
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
H4sIAAAJbogA/7RXTY/bNhA9W7+CEJpbl4rSGKhTWpdskB7qdFO3PfRGiWOLqES65HidreD/XpD6sCxr
Ha+KXCzpkW/IN/M0lFmOZZGwHLhIGEosIPkI+v7TmlQVoX+CsVIrcjyyqB4MWCHV3yQ3sFmGUaQAheI0
1RotGr7LhKKZLiM8SEQwd91A9Ib+QOMoszbqsLtMl6lUIGgpFc2sDYmBYhlafCrA5gAY3rqeBw4cs7xd
CMy+AK5Oq11dxD8lwQwF/VcrULyE71FQlxYwpCIbrfDuAHKb4zuS6kL8RI4BixoaS7V4SoKACflIsoJb
uwwzrZBLBSZ0A3mcfCx0ygsW5fFgptEHN2fWx+yOqx89OmPI0wLagfrB/96l2ggwIJrHTCsByoIIk2Dm
eMZfZwxFy5Zqo0ktKmw2RCxytCxC0c1OYlJKdQ7NL6F4BFsBPyEsclsY7iX5vAcjYbBkVe2MVLgh4Sv6
ZhMSWu+ONpPpbxwh9jZ8IWk+hRRPYjnxPV4t319dfb5tMX+WFvXW8HKQ19XVCnlk8frVAFhcAHQIrfiX
c2CN4h4ev1b7xnP/fMUCIiSkS3Enja6kurUuPdJ5VW5kPWS4eE2m8RZTeVeJz+WEf3n5anWxyIVZz93K
IiEfxzrT2//VmVpD9KzcuviPHcoSTpvyYmiN0nv+tEYj1fa07e4Vez7kQ8Fxo005DNriI8EuM9Bd8zj5
SyvnXNfGJyXA75VhnrAI8/rO99s9wgmZN4jtTRrBnLfJ54d1g9Q1rCryHerdrzt0R/e7JaG/t0/Hoxs2
XG2BEOqVEIe5BPbbSlMnQTJduIIv5+1oezqGLoefeAld/loHsbYkF51+1JsrQCMz+1yjv5kzn8CJp5DO
+smI6A/u4+klyj8IZaeoH/JuFXOx3lTiaCac+zK9V2i99VruLzyFYu0+NZwb39cTejb1LqwqIjcd/Xis
3+wLY547M/Gh7fAtbm1+Fm1wjjYh3jov+yiNmqZF+F2OHOhVBUq4eO1No7qQMJT93mO36m4C3Cq8Dn5F
eT/et5IedHenvnlqm/WHMYv8v4zgvwAAAP//sJ1SyW0MAAA=
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
