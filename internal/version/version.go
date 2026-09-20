package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type Info struct {
	Version string `json:"version" yaml:"version"`
	Commit  string `json:"commit" yaml:"commit"`
	Date    string `json:"date" yaml:"date"`
}

func Current() Info { return Info{Version: Version, Commit: Commit, Date: Date} }
