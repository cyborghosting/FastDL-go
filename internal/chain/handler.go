package chain

type State struct {
	InstallationPath string
	Dictionary       map[string]string
	CachePath        string
	CompressMaxSize  int64

	GameInfo           GameInfoState
	SearchPath         SearchPathState
	FilteredSearchPath FilteredSearchPathState
	ResolvedSearchPath ResolvedSearchPathState
	OverlayFs          OverlayFsState
	CacheFs            CacheFsState
}

type Handler interface {
	Handle(*State)
	SetNext(Handler)
}
