package storage

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/marpio/mirror/crypto"
	"github.com/marpio/mirror/remotestorage/filesystem"
	"github.com/spf13/afero"
)

const encKey = "b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"

var ctx context.Context = context.Background()

type readCloser struct {
	io.Reader
}

func (readCloser) Close() error { return nil }

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func TestWriteRead(t *testing.T) {
	afs := afero.NewMemMapFs()
	b := filesystem.New(afs)
	c := crypto.NewService(encKey)
	rs := New(b, c)

	path1 := "path1"
	sizes := []int{10, 1000, 65536, 80000, 131072, 328000, 1234567}
	rand.Seed(time.Now().Unix())
	for _, s := range sizes {
		data := make([]byte, s)
		for i := 0; i < s; i++ {
			data[i] = byte(rand.Intn(256))
		}
		w := rs.NewWriter(ctx, path1)

		io.Copy(w, bytes.NewReader(data[:]))
		w.Close()

		r, _ := rs.NewReader(ctx, path1)
		var dst bytes.Buffer
		_, err := io.Copy(&dst, r)
		if err != nil {
			t.Errorf("error reading from remote storage: %v", err.Error())
		}
		r.Close()

		f, _ := afs.Open(path1)
		res, _ := ioutil.ReadAll(f)
		blockSize := c.BlockSize()
		addOne := len(data)%blockSize != 0
		var mult int
		if addOne {
			mult = len(data)/blockSize + 1
		} else {
			mult = len(data) / blockSize
		}
		expectedLen := mult*(c.NonceSize()+c.Overhead()) + len(data)
		actualLen := len(res)
		if actualLen != expectedLen {
			t.Errorf("expected len of the uploaded data: %v, actual: %v. data not written or encryption broken.", expectedLen, actualLen)
		}

		if !bytes.Equal(data[:], dst.Bytes()) {
			t.Error("downloaded data does not match the uploaded.")
		}
	}
}
