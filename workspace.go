package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ScanChunk struct {
	ImageKeys  []string    `json:"image_keys"`
	GaniFiles  [][2]string `json:"gani_files"`
	SoundFiles []string    `json:"sound_files"`
	LevelFiles [][2]string `json:"level_files"`
	Done       bool        `json:"done"`
	ImageCount int         `json:"image_count"`
	GaniCount  int         `json:"gani_count"`
}

type CacheResult struct {
	Dir     string  `json:"dir"`
	Entries [][]any `json:"entries"`
}

func (s *AppService) ScanWorkspace(dir string) error {
	s.mu.Lock()
	if s.scanRunning {
		s.mu.Unlock()
		return nil
	}
	s.index = make(map[string]string)
	s.lastDir = dir
	s.rescanDone = false
	s.scanRunning = true
	s.mu.Unlock()
	go func() {
		s.scanDirectory(dir, true)
		s.mu.Lock()
		s.scanRunning = false
		s.mu.Unlock()
	}()
	return nil
}

func (s *AppService) LoadWorkspaceCache() (*CacheResult, error) {
	cachePath := filepath.Join(s.exeDir, "FILENAMECACHE.txt")
	content, err := os.ReadFile(cachePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dir := ""
	entries := make([][]any, 0)
	index := make(map[string]string)
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "DIR|") {
			dir = strings.TrimPrefix(line, "DIR|")
			continue
		}
		comma := strings.LastIndexByte(line, ',')
		if comma <= 0 {
			continue
		}
		rel := line[:comma]
		checksum, parseErr := strconv.ParseUint(strings.TrimSpace(line[comma+1:]), 10, 32)
		if parseErr != nil {
			continue
		}
		name := strings.ToLower(filepath.Base(rel))
		full := filepath.Join(dir, rel)
		entries = append(entries, []any{name, full, uint32(checksum)})
		if name != "" {
			index[name] = full
		}
	}
	if dir == "" {
		return nil, nil
	}
	s.mu.Lock()
	s.index = index
	s.lastDir = dir
	s.rescanDone = false
	if !s.scanRunning {
		s.scanRunning = true
		go s.rescanWorkspace(dir)
	}
	s.mu.Unlock()
	return &CacheResult{Dir: dir, Entries: entries}, nil
}

func (s *AppService) scanDirectory(dir string, emitProgress bool) {
	const chunkSize = 500
	imageKeys := make([]string, 0, chunkSize)
	ganiFiles := make([][2]string, 0)
	soundFiles := make([]string, 0)
	levelFiles := make([][2]string, 0)
	batch := make([][2]string, 0, chunkSize)
	imageCount := 0
	ganiCount := 0
	base := filepath.Clean(dir)
	cacheFile, _ := os.Create(filepath.Join(s.exeDir, "FILENAMECACHE.txt"))
	if cacheFile != nil {
		defer cacheFile.Close()
		_, _ = fmt.Fprintf(cacheFile, "DIR|%s\n", dir)
	}
	flush := func(done bool) {
		s.updateIndex(batch)
		if emitProgress {
			s.emit("workspace_chunk", ScanChunk{ImageKeys: imageKeys, GaniFiles: ganiFiles, SoundFiles: soundFiles, LevelFiles: levelFiles, Done: done, ImageCount: imageCount, GaniCount: ganiCount})
		}
		imageKeys = make([]string, 0, chunkSize)
		ganiFiles = make([][2]string, 0)
		soundFiles = make([]string, 0)
		levelFiles = make([][2]string, 0)
		batch = make([][2]string, 0, chunkSize)
	}
	_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		full := filepath.Clean(path)
		relevant := false
		switch {
		case isImageExtension(ext):
			imageKeys = append(imageKeys, name)
			batch = append(batch, [2]string{name, full})
			imageCount++
			relevant = true
		case ext == "gani":
			ganiFiles = append(ganiFiles, [2]string{name, full})
			batch = append(batch, [2]string{name, full})
			ganiCount++
			relevant = true
		case isSoundExtension(ext):
			soundFiles = append(soundFiles, full)
			relevant = true
		case isLevelExtension(ext):
			levelFiles = append(levelFiles, [2]string{name, full})
			batch = append(batch, [2]string{name, full})
			relevant = true
		}
		if relevant && cacheFile != nil {
			rel, relErr := filepath.Rel(base, full)
			if relErr == nil {
				_, _ = fmt.Fprintf(cacheFile, "%s,%d\n", rel, crc32.ChecksumIEEE([]byte(rel)))
			}
		}
		if emitProgress && len(batch) >= chunkSize {
			flush(false)
		}
		return nil
	})
	flush(true)
}

func (s *AppService) rescanWorkspace(dir string) {
	time.Sleep(2 * time.Second)
	current := walkRelevantFiles(dir)
	s.mu.RLock()
	oldKeys := make([]string, 0, len(s.index))
	for name := range s.index {
		oldKeys = append(oldKeys, name)
	}
	s.mu.RUnlock()
	newKeys := make([]string, 0, len(current))
	for _, entry := range current {
		newKeys = append(newKeys, entry[0])
	}
	sort.Strings(oldKeys)
	sort.Strings(newKeys)
	if !sameStrings(oldKeys, newKeys) {
		index := make(map[string]string)
		for _, entry := range current {
			index[entry[0]] = entry[1]
		}
		s.mu.Lock()
		s.index = index
		s.mu.Unlock()
		s.writeCache(dir, current)
		imageKeys := make([]string, 0)
		ganiFiles := make([][2]string, 0)
		soundFiles := make([]string, 0)
		levelFiles := make([][2]string, 0)
		imageCount := 0
		ganiCount := 0
		for _, entry := range current {
			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(entry[0])), ".")
			switch {
			case isImageExtension(ext):
				imageKeys = append(imageKeys, entry[0])
				imageCount++
			case ext == "gani":
				ganiFiles = append(ganiFiles, entry)
				ganiCount++
			case isLevelExtension(ext):
				levelFiles = append(levelFiles, entry)
			case isSoundExtension(ext):
				soundFiles = append(soundFiles, entry[1])
			}
		}
		s.emit("workspace_chunk", ScanChunk{ImageKeys: imageKeys, GaniFiles: ganiFiles, SoundFiles: soundFiles, LevelFiles: levelFiles, Done: true, ImageCount: imageCount, GaniCount: ganiCount})
	}
	s.mu.Lock()
	s.scanRunning = false
	s.rescanDone = true
	s.mu.Unlock()
}

func (s *AppService) updateIndex(entries [][2]string) {
	if len(entries) == 0 {
		return
	}
	s.mu.Lock()
	for _, entry := range entries {
		s.index[entry[0]] = entry[1]
	}
	s.mu.Unlock()
}

func (s *AppService) writeCache(dir string, entries [][2]string) {
	file, err := os.Create(filepath.Join(s.exeDir, "FILENAMECACHE.txt"))
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "DIR|%s\n", dir)
	base := filepath.Clean(dir)
	for _, entry := range entries {
		rel, relErr := filepath.Rel(base, entry[1])
		if relErr == nil {
			_, _ = fmt.Fprintf(file, "%s,%d\n", rel, crc32.ChecksumIEEE([]byte(rel)))
		}
	}
}

func walkRelevantFiles(dir string) [][2]string {
	result := make([][2]string, 0)
	_ = filepath.WalkDir(filepath.Clean(dir), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		if isImageExtension(ext) || ext == "gani" || isSoundExtension(ext) || isLevelExtension(ext) {
			result = append(result, [2]string{name, filepath.Clean(path)})
		}
		return nil
	})
	return result
}

func isImageExtension(ext string) bool {
	switch ext {
	case "png", "gif", "jpg", "jpeg", "webp", "bmp", "mng":
		return true
	default:
		return false
	}
}

func isSoundExtension(ext string) bool {
	switch ext {
	case "wav", "mp3", "ogg", "mid", "midi":
		return true
	default:
		return false
	}
}

func isLevelExtension(ext string) bool {
	switch ext {
	case "nw", "gmap", "graal", "zelda":
		return true
	default:
		return false
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
