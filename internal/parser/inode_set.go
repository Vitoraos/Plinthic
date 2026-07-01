package parser

import "os"

// Tracks visited dirs for symlink-cycle detection (portable via os.SameFile).
type inodeSet struct {
	infos []os.FileInfo
}

func newInodeSet() *inodeSet {
	return &inodeSet{}
}

func (s *inodeSet) isVisited(path string, info os.FileInfo) bool {
	for _, existing := range s.infos {
		if os.SameFile(existing, info) {
			return true
		}
	}
	return false
}

func (s *inodeSet) markVisited(path string, info os.FileInfo) {
	s.infos = append(s.infos, info)
}
