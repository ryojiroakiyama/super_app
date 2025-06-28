package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"gmail-tts-app/internal/domain/audio"
)

// FileStore saves audio bytes to local directory (default audios/).
type FileStore struct {
	Dir string
}

func NewFileStore(dir string) *FileStore {
	if dir == "" {
		dir = "audio"
	}
	return &FileStore{Dir: dir}
}

// Save writes data to {dir}/{fileName}.mp3 and returns the path.
func (fs *FileStore) Save(data []byte, fileName string) (audio.Path, error) {
	if err := os.MkdirAll(fs.Dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(fs.Dir, fmt.Sprintf("%s.mp3", fileName))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return audio.Path(path), nil
}
