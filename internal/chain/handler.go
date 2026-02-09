package chain

type State struct {
	InstallationPath string
	Dictionary       map[string]string
	CachePath        string
	CompressMaxSize  int64

	ParseGameInfo     ParseGameInfoState
	ParseSearchPath   ParseSearchPathState
	FilterSearchPath  FilterSearchPathState
	CollectSearchPath CollectSearchPathState
	BuildFileSystem   BuildFileSystemState
	CacheFileSystem   CacheFileSystemState
}

type Handler interface {
	Handle(*State)
	SetNext(Handler)
}
