package utils

import (
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/PuerkitoBio/goquery"
)

type SafeQuery struct {
	mut   sync.RWMutex
	inner *goquery.Document
}

func NewSafeQuery(head *goquery.Document) *SafeQuery {
	return &SafeQuery{
		inner: head,
	}
}

func (sq *SafeQuery) Read() *goquery.Document {
	sq.mut.RLock()
	defer sq.mut.RUnlock()
	return sq.inner
}

func (sq *SafeQuery) Write(head *goquery.Document) {
	sq.mut.Lock()
	defer sq.mut.Unlock()
	sq.inner = head
}

type ShortCache struct {
	// set type: map[string]*atomic.Bool
	set sync.Map
}

func NewCache() *ShortCache {
	return &ShortCache{}
}

func (sc *ShortCache) Get(key string) bool {
	v, exists := sc.set.Load(key)
	if !exists {
		return false
	}
	return v.(*atomic.Bool).Load()
}

func (sc *ShortCache) Add(key string) {
	nv, exists := sc.set.LoadOrStore(key, new(atomic.Bool))
	if !exists {
		nv.(*atomic.Bool).Store(true)
	}
}

func RequestURL(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func WriteFile(URL string, outPath string) error {
	resp, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return os.WriteFile(outPath, data, 0666)
}

func CreateDirectory(path string) error {
	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}
	return nil
}
