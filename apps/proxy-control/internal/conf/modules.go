package conf

type Modules struct {
	ProxyAuth bool
	Usage     bool
}

func NewModules(settings *Data) Modules {
	repo := settings.GetRepo()
	return Modules{ProxyAuth: repo.GetProxyAuth() != nil, Usage: repo.GetUsage() != nil}
}
