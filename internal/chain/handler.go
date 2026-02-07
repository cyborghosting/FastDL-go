package chain

type State struct {
	InstallationPath string
	Dictionary       map[string]string

	ParseGameInfo     ParseGameInfoState
	ParseSearchPath   ParseSearchPathState
	FilterSearchPath  FilterSearchPathState
	CollectSearchPath CollectSearchPathState
	BuildFileSystem   BuildFileSystemState
}

type Handler interface {
	Handle(*State)
	SetNext(Handler)
}
