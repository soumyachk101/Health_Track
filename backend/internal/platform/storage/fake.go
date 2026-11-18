package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// FakeObjectStorage is the in-memory ObjectStorage for tests: a map of key →
// object with the same NotFound semantics as R2. It records presign mints
// so a test can assert TTLs without a network.
type FakeObjectStorage struct {
	mu      sync.Mutex
	objects map[string]fakeObject

	Puts    []fakePut // server-side PutObject calls (confirm/cleanup paths)
	PresPut []string  // PresignPut mints (upload-url path)
	PresGet []string  // PresignGet mints (download-url path)
	Delet   []string  // DeleteObject calls (cleanup path)
}

type fakeObject struct {
	body         []byte
	contentType  string
	lastModified time.Time
}

type fakePut struct {
	Key         string
	ContentType string
}

// NewFakeObjectStorage returns an empty fake.
func NewFakeObjectStorage() *FakeObjectStorage {
	return &FakeObjectStorage{objects: map[string]fakeObject{}}
}

func (f *FakeObjectStorage) PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = fakeObject{body: append([]byte(nil), body...), contentType: contentType, lastModified: time.Now()}
	f.Puts = append(f.Puts, fakePut{Key: key, ContentType: contentType})
	return nil
}

func (f *FakeObjectStorage) HeadObject(ctx context.Context, key string) (*ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return &ObjectInfo{
		Key:          key,
		Size:         int64(len(o.body)),
		ContentType:  o.contentType,
		LastModified: o.lastModified,
	}, nil
}

func (f *FakeObjectStorage) GetObject(ctx context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), o.body...), nil
}

func (f *FakeObjectStorage) DeleteObject(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
	f.Delet = append(f.Delet, key)
	return nil
}

func (f *FakeObjectStorage) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.PresPut = append(f.PresPut, key)
	return fmt.Sprintf("https://r2.test/%s?put=1&ttl=%d&ct=%s", key, int64(ttl/time.Second), contentType), nil
}

func (f *FakeObjectStorage) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.PresGet = append(f.PresGet, key)
	return fmt.Sprintf("https://r2.test/%s?get=1&ttl=%d", key, int64(ttl/time.Second)), nil
}

// ObjectKeys returns the stored keys, sorted.
func (f *FakeObjectStorage) ObjectKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
