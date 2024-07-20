package models

type FileInfo struct {
	Parent   string      `json:"parent"`
	Name     string      `json:"name"`
	IsDir    bool        `json:"is_dir"`
	Children []*FileInfo `json:"children,omitempty"`
}
