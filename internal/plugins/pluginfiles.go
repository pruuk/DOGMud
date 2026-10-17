package plugins

import (
	"embed"
	"io/fs"
	"path/filepath"
)

// Implements fs.ReadFileFS
type PluginFiles struct {
	fileSystem embed.FS
	filePaths  map[string]string
}

// embedPath maps a datafiles-relative name to the embedded path it lives at.
// The keys are embed paths, which always use forward slashes, while callers
// build names with util.FilePath, which uses the OS separator. On Windows
// every lookup used to miss, so no module's help topics, keyword overlays or
// config overlays loaded there (`help follow` found nothing on a Windows
// server, 2026-10-17). ToSlash is a no-op on Linux.
func (p PluginFiles) embedPath(name string) (string, bool) {
	embedPath, ok := p.filePaths[filepath.ToSlash(name)]
	return embedPath, ok
}

func (p PluginFiles) ReadFile(name string) ([]byte, error) {

	if embedPath, ok := p.embedPath(name); ok {
		b, err := p.fileSystem.ReadFile(embedPath)
		if err == nil {
			return b, nil
		}
	}

	return nil, fs.ErrNotExist
}

func (p PluginFiles) Open(name string) (fs.File, error) {

	if embedPath, ok := p.embedPath(name); ok {
		return p.fileSystem.Open(embedPath)

	}

	return nil, fs.ErrNotExist

}

func (p PluginFiles) Stat(name string) (fs.FileInfo, error) {

	if embedPath, ok := p.embedPath(name); ok {
		return fs.Stat(p.fileSystem, embedPath)
	}

	return nil, fs.ErrNotExist

}
