package audio

// Path is the saved location of audio file (local path or URL).
type Path string

// Store persists synthesized audio.
type Store interface {
	// Save persists data with given file name (without extension) and returns the saved path.
	Save(data []byte, fileName string) (Path, error)
}
