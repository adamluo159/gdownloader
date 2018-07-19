package gDownloder

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type fblock struct {
	begin   int64
	end     int64
	number  int
	size    int64
	curSize int64
}

type fdonwload struct {
	url string

	blocks      []*fblock
	f           *os.File
	fsize       int64
	grouplocker sync.WaitGroup
}

var (
	buflen int   = 1024 * 30
	goNum  int64 = 300
)

func (fdl *fdonwload) blockDownload(number int) {
	defer fdl.grouplocker.Done()
	bl := fdl.blocks[number]

	buf := make([]byte, buflen)
	req, _ := http.NewRequest("GET", fdl.url, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", bl.begin, bl.end))

	for {
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		curPos := bl.begin
		for {
			n, err := io.ReadFull(r.Body, buf)
			fdl.f.WriteAt(buf[:n], curPos)
			curPos += int64(n)
			bl.curSize += int64(n)
			if err != nil {
				break
			}
		}
		r.Body.Close()

		if bl.curSize != r.ContentLength {
			log.Printf("block:%+v, contentLen:%d", bl, r.ContentLength)
			//continue
		}
		break
	}
}

func createFile(url string) (*os.File, error) {
	a := strings.Split(url, "/")
	i := len(a) - 1
	countStr := strings.Count(a[i], "") - 1
	if countStr > 30 {
		a[i] = string([]rune(a[i])[countStr-30:])
	}
	return os.Create(a[i])
}

func Download(url string) error {
	count := 0
	fdl := &fdonwload{
		url:    url,
		blocks: make([]*fblock, 0, int(goNum)),
	}

	for {
		if count > 10 {
			return fmt.Errorf("over get count 10")
		}
		r, err := http.Get(url)
		if err != nil {
			log.Printf("fdl new error:%+v", err)
			count++
			continue
		}
		log.Printf("head: %+v", r.Header)
		fdl.fsize = r.ContentLength
		r.Body.Close()
		break
	}
	f, err := createFile(url)
	if err != nil {
		return err
	}
	fdl.f = f
	blockSize := fdl.fsize / goNum

	fdl.grouplocker.Add(int(goNum))
	for i := int64(0); i < goNum-1; i++ {
		bl := &fblock{
			begin:  i * blockSize,
			size:   blockSize,
			end:    i*blockSize + blockSize,
			number: int(i),
		}
		fdl.blocks = append(fdl.blocks, bl)
		go fdl.blockDownload(bl.number)
	}
	bl := &fblock{
		begin:  (goNum - 1) * blockSize,
		size:   blockSize + (fdl.fsize % goNum),
		number: int(goNum - 1),
	}
	bl.end = bl.begin + bl.size
	fdl.blocks = append(fdl.blocks, bl)
	go fdl.blockDownload(bl.number)

	fdl.grouplocker.Wait()
	return nil
}
