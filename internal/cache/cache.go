package cache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	cacheVersion    = 1
	defaultTTLDays  = 30
	cacheDirName    = "lyric-shower"
	lyricsCacheName = "lyrics"
)

var (
	ErrCacheMiss    = errors.New("cache miss")
	ErrCacheExpired = errors.New("cache expired")
	ErrCacheCorrupt = errors.New("cache corrupt")
)

type LyricEntry struct {
	Version      uint8
	TrackName    string
	ArtistName   string
	AlbumName    string
	Duration     float64
	Instrumental bool
	PlainLyrics  string
	SyncedLyrics string
	SyncOffset   float64
	CreatedAt    int64
	ExpiresAt    int64
}

type DiskCache struct {
	basePath string
	mu       sync.RWMutex
	memCache map[string]*LyricEntry
}

var (
	globalCache     *DiskCache
	globalCacheOnce sync.Once
)

func GetGlobalCache() *DiskCache {
	globalCacheOnce.Do(func() {
		cache, err := NewDiskCache()
		if err != nil {
			cache = &DiskCache{
				basePath: "",
				memCache: make(map[string]*LyricEntry),
			}
		}
		globalCache = cache
	})
	return globalCache
}

func NewDiskCache() (*DiskCache, error) {
	cacheDir, err := getCacheDirectory()
	if err != nil {
		return nil, err
	}

	lyricsPath := filepath.Join(cacheDir, lyricsCacheName)
	err = os.MkdirAll(lyricsPath, 0755)
	if err != nil {
		return nil, err
	}

	return &DiskCache{
		basePath: lyricsPath,
		memCache: make(map[string]*LyricEntry),
	}, nil
}

func getCacheDirectory() (string, error) {
	// xdg cache home takes priority
	xdgCache := os.Getenv("XDG_CACHE_HOME")
	if xdgCache != "" {
		return filepath.Join(xdgCache, cacheDirName), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".cache", cacheDirName), nil
}

func generateKey(artist, title string) string {
	normalized := strings.ToLower(artist) + "|" + strings.ToLower(title)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:12])
}

func (c *DiskCache) getFilePath(key string) string {
	if c.basePath == "" {
		return ""
	}
	return filepath.Join(c.basePath, key+".bin")
}

func (c *DiskCache) Get(artist, title string) (*LyricEntry, error) {
	if artist == "" || title == "" {
		return nil, ErrCacheMiss
	}

	key := generateKey(artist, title)

	// check memory cache first
	c.mu.RLock()
	entry, exists := c.memCache[key]
	c.mu.RUnlock()

	if exists {
		if entry.ExpiresAt > time.Now().Unix() {
			return entry, nil
		}
		// expired in memory, remove it
		c.mu.Lock()
		delete(c.memCache, key)
		c.mu.Unlock()
	}

	// fall back to disk cache
	if c.basePath == "" {
		return nil, ErrCacheMiss
	}

	filePath := c.getFilePath(key)
	entry, err := c.readFromDisk(filePath)
	if err != nil {
		return nil, err
	}

	// validate expiry
	if entry.ExpiresAt <= time.Now().Unix() {
		_ = os.Remove(filePath)
		return nil, ErrCacheExpired
	}

	// populate memory cache
	c.mu.Lock()
	c.memCache[key] = entry
	c.mu.Unlock()

	return entry, nil
}

func (c *DiskCache) Set(artist, title string, entry *LyricEntry) error {
	if artist == "" || title == "" || entry == nil {
		return errors.New("invalid cache entry")
	}

	key := generateKey(artist, title)

	// set timestamps
	now := time.Now().Unix()
	entry.Version = cacheVersion
	entry.CreatedAt = now
	entry.ExpiresAt = now + int64(defaultTTLDays*24*60*60)

	// store in memory
	c.mu.Lock()
	c.memCache[key] = entry
	c.mu.Unlock()

	// persist to disk
	if c.basePath == "" {
		return nil
	}

	return c.writeToDisk(c.getFilePath(key), entry)
}

func (c *DiskCache) readFromDisk(filePath string) (*LyricEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, err
	}
	defer file.Close()

	var entry LyricEntry
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&entry)
	if err != nil {
		return nil, ErrCacheCorrupt
	}

	// version mismatch means stale format
	if entry.Version != cacheVersion {
		_ = os.Remove(filePath)
		return nil, ErrCacheCorrupt
	}

	return &entry, nil
}

func (c *DiskCache) writeToDisk(filePath string, entry *LyricEntry) error {
	// write to temp file first, then rename for atomicity
	tmpPath := filePath + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(entry)
	if err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	err = file.Sync()
	if err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	err = file.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, filePath)
}

func (c *DiskCache) Clear() error {
	c.mu.Lock()
	c.memCache = make(map[string]*LyricEntry)
	c.mu.Unlock()

	if c.basePath == "" {
		return nil
	}

	entries, err := os.ReadDir(c.basePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".bin") {
			_ = os.Remove(filepath.Join(c.basePath, entry.Name()))
		}
	}

	return nil
}

func (c *DiskCache) Prune() (int, error) {
	if c.basePath == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(c.basePath)
	if err != nil {
		return 0, err
	}

	pruned := 0
	now := time.Now().Unix()

	for _, dirEntry := range entries {
		if dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), ".bin") {
			continue
		}

		filePath := filepath.Join(c.basePath, dirEntry.Name())
		entry, err := c.readFromDisk(filePath)
		if err != nil {
			_ = os.Remove(filePath)
			pruned++
			continue
		}

		if entry.ExpiresAt <= now {
			_ = os.Remove(filePath)
			pruned++
		}
	}

	return pruned, nil
}

func (c *DiskCache) Stats() (count int, sizeBytes int64, err error) {
	if c.basePath == "" {
		return 0, 0, nil
	}

	entries, err := os.ReadDir(c.basePath)
	if err != nil {
		return 0, 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bin") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		count++
		sizeBytes += info.Size()
	}

	return count, sizeBytes, nil
}

func (c *DiskCache) ListAll() ([]*LyricEntry, error) {
	if c.basePath == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(c.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []*LyricEntry

	for _, dirEntry := range entries {
		if dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), ".bin") {
			continue
		}

		filePath := filepath.Join(c.basePath, dirEntry.Name())
		entry, err := c.readFromDisk(filePath)
		if err != nil {
			continue
		}

		result = append(result, entry)
	}

	return result, nil
}

func (c *DiskCache) Delete(artist, title string) error {
	if artist == "" || title == "" {
		return errors.New("invalid artist or title")
	}

	key := generateKey(artist, title)

	// remove from memory cache
	c.mu.Lock()
	delete(c.memCache, key)
	c.mu.Unlock()

	// remove from disk
	if c.basePath == "" {
		return nil
	}

	filePath := c.getFilePath(key)
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
